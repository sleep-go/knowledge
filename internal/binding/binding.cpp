#include "binding.h"
#include "chat.h"
#include "common.h"
#include "llama.h"
#include "sampling.h"

#include <cstring>
#include <memory>
#include <nlohmann/json.hpp>
#include <string>
#include <vector>
#include <algorithm>
#include <limits>
#include <exception>

struct LlamaBindingContext {
    std::unique_ptr<common_init_result> init_res;
    common_chat_templates_ptr chat_tmpls;
    llama_model * model = nullptr;
    llama_context * ctx = nullptr;
};

static std::vector<std::string> split_unit_sep(const char * s) {
    std::vector<std::string> out;
    if (s == nullptr || s[0] == '\0') {
        return out;
    }
    const char sep = 0x1f;
    std::string cur;
    for (size_t i = 0; s[i] != '\0'; i++) {
        if (s[i] == sep) {
            out.push_back(cur);
            cur.clear();
        } else {
            cur.push_back(s[i]);
        }
    }
    out.push_back(cur);
    return out;
}

static bool build_chat_prompt(LlamaBindingContext * bctx, const char * messages_json, std::string & out_prompt, std::vector<std::string> & out_stops) {
    if (messages_json == nullptr || messages_json[0] == '\0') {
        return false;
    }

    nlohmann::ordered_json j = nlohmann::ordered_json::parse(messages_json, nullptr, false);
    if (j.is_discarded() || !j.is_array()) {
        return false;
    }

    common_chat_templates_inputs inputs;
    inputs.messages = common_chat_msgs_parse_oaicompat(j);
    inputs.add_generation_prompt = true;
    inputs.use_jinja = true;
    inputs.add_bos = true;
    inputs.add_eos = false;

    try {
        const common_chat_params chat = common_chat_templates_apply(bctx->chat_tmpls.get(), inputs);
        out_prompt = chat.prompt;
        out_stops = chat.additional_stops;
        return true;
    } catch (const std::exception &) {
    } catch (...) {
    }

    inputs.use_jinja = false;

    try {
        const common_chat_params chat = common_chat_templates_apply(bctx->chat_tmpls.get(), inputs);
        out_prompt = chat.prompt;
        out_stops = chat.additional_stops;
        return true;
    } catch (...) {
        return false;
    }
}

static size_t string_find_partial_stop(const std::string & result, const std::string & stop) {
    // 安全的部分停止字符串查找函数
    if (stop.empty() || result.empty()) {
        return std::string::npos;
    }
    
    // 检查 result 是否以 stop 的前缀开始
    if (result.size() < stop.size()) {
        // 检查 result 是否是 stop 的前缀
        if (stop.substr(0, result.size()) == result) {
            return 0; // 在开头找到部分匹配
        }
    }
    
    return std::string::npos;
}

static bool chat_generate(
        LlamaBindingContext * bctx,
        const std::string & prompt,
        const std::vector<std::string> & stop_strs,
        int n_predict,
        float temp,
        float top_p,
        int top_k,
        float repeat_penalty,
        uintptr_t cb_handle,
        std::string * out_result) {

    common_params_sampling sparams;
    sparams.temp = temp;
    sparams.top_p = top_p;
    sparams.top_k = top_k;
    sparams.penalty_repeat = repeat_penalty;

    common_sampler * sampler = common_sampler_init(bctx->model, sparams);
    if (sampler == nullptr) {
        fprintf(stderr, "[llama_binding] Error: failed to init sampler\n");
        return false;
    }

    std::vector<llama_token> tokens_list = common_tokenize(bctx->ctx, prompt, true, true);
    const uint32_t n_ctx = llama_n_ctx(bctx->ctx);
    fprintf(stderr, "[llama_binding] Token count: %zu, context size: %u\n", tokens_list.size(), n_ctx);
    
    if (n_ctx > 0 && tokens_list.size() > (size_t) n_ctx) {
        std::vector<llama_token> trimmed;
        trimmed.reserve(n_ctx);
        trimmed.push_back(tokens_list.front());
        if (n_ctx > 1) {
            trimmed.insert(trimmed.end(), tokens_list.end() - (n_ctx - 1), tokens_list.end());
        }
        tokens_list.swap(trimmed);
        fprintf(stderr, "[llama_binding] Trimmed token count: %zu\n", tokens_list.size());
    }

    llama_memory_seq_rm(llama_get_memory(bctx->ctx), 0, -1, -1);

    // 使用与 context 创建时相同的 batch size 进行分块处理
    const uint32_t n_batch = llama_n_batch(bctx->ctx);
    llama_batch batch = llama_batch_init(n_batch, 0, 1);

    // 分块处理 prompt，避免一次性提交过多 token 导致内存不足
    for (size_t i = 0; i < tokens_list.size(); i += n_batch) {
        common_batch_clear(batch);
        
        size_t n_eval = tokens_list.size() - i;
        if (n_eval > n_batch) {
            n_eval = n_batch;
        }
        
        for (size_t j = 0; j < n_eval; j++) {
            common_batch_add(batch, tokens_list[i + j], (int32_t)(i + j), { 0 }, false);
        }
        
        // 只有最后一个 chunk 的最后一个 token 需要 logits
        if (i + n_eval == tokens_list.size()) {
            batch.logits[batch.n_tokens - 1] = true;
        }
        
        if (llama_decode(bctx->ctx, batch) != 0) {
            fprintf(stderr, "[llama_binding] Error: llama_decode failed during prompt processing at offset %zu, batch size: %u, n_eval: %zu\n", i, n_batch, n_eval);
            common_sampler_free(sampler);
            llama_batch_free(batch);
            return false;
        }
    }

    const llama_vocab * vocab = llama_model_get_vocab(bctx->model);

    std::string result;
    size_t sent_len = 0;

    int n_cur = tokens_list.size();
     const int n_input = n_cur;
     // n_ctx 已经在前面定义过了，不需要重新定义
     
     fprintf(stderr, "[llama_binding] Starting generation loop, n_input: %d, n_predict: %d, n_ctx: %u\n", n_input, n_predict, n_ctx);
     fflush(stderr);
    
    // 循环条件：
    // 1. 生成数量不超过 n_predict (如果 n_predict >= 0)
    // 2. 总长度不超过 n_ctx
    while ((n_predict < 0 || (n_cur - n_input) < n_predict) && (uint32_t)n_cur < n_ctx) {
        const llama_token new_token_id = common_sampler_sample(sampler, bctx->ctx, -1);
        if (new_token_id < 0) {
            fprintf(stderr, "[llama_binding] Error: sampler_sample returned invalid token\n");
            break;
        }
        
        fprintf(stderr, "[llama_binding] Token sampled: %d\n", new_token_id);
        common_sampler_accept(sampler, new_token_id, true);

        if (llama_vocab_is_eog(vocab, new_token_id)) {
            fprintf(stderr, "[llama_binding] End of generation token detected\n");
            break;
        }
        
        // 检查是否超出 n_ctx
        if ((uint32_t)n_cur >= n_ctx) {
            fprintf(stderr, "[llama_binding] Context limit reached (%u), stopping generation\n", n_ctx);
            break;
        }

        const std::string piece = common_token_to_piece(bctx->ctx, new_token_id, false);
        if (!piece.empty()) {
            result.append(piece);
            fprintf(stderr, "[llama_binding] Got piece: %s (total len: %zu)\n", piece.c_str(), result.size());
        }

        size_t safe_len = result.size();
        bool should_stop = false;

        for (const auto & stop : stop_strs) {
            if (stop.empty()) {
                continue;
            }

            const size_t stop_pos = result.find(stop);
            if (stop_pos != std::string::npos) {
                safe_len = std::min(safe_len, stop_pos);
                should_stop = true;
                continue;
            }

            const size_t partial_pos = string_find_partial_stop(result, stop);
            if (partial_pos != std::string::npos) {
                safe_len = std::min(safe_len, partial_pos);
            }
        }

        if (cb_handle != 0 && safe_len > sent_len) {
            const std::string delta = result.substr(sent_len, safe_len - sent_len);
            sent_len = safe_len;
            if (!delta.empty()) {
                fprintf(stderr, "[llama_binding] Calling callback with delta: %s\n", delta.c_str());
                fflush(stderr);
                const int keep_going = llama_binding_go_on_token(cb_handle, const_cast<char *>(delta.c_str()));
                if (keep_going == 0) {
                    fprintf(stderr, "[llama_binding] Callback requested stop\n");
                    should_stop = true;
                }
            }
        }

        if (should_stop) {
            result.resize(safe_len);
            break;
        }

        common_batch_clear(batch);
        common_batch_add(batch, new_token_id, n_cur, { 0 }, true);
        n_cur++;

        if (llama_decode(bctx->ctx, batch) != 0) {
            break;
        }
    }
    
    fprintf(stderr, "[llama_binding] Generation completed. Total tokens generated: %d\n", n_cur - n_input);

    if (cb_handle != 0 && sent_len < result.size()) {
        const std::string delta = result.substr(sent_len);
        if (!delta.empty()) {
            llama_binding_go_on_token(cb_handle, const_cast<char *>(delta.c_str()));
        }
    }

    common_sampler_free(sampler);
    llama_batch_free(batch);

    if (out_result) {
        *out_result = result;
    }

    return true;
}

extern "C" {

void * llama_binding_load_model(const char * model_path, int n_ctx, int n_threads, int n_gpu_layers) {
    auto * bctx = new LlamaBindingContext();

    common_params params;
    params.model.path = model_path;
    params.n_ctx = n_ctx;
    params.cpuparams.n_threads = n_threads;
    params.n_batch = 512; // 显式设置 batch size，避免默认值过大或过小导致问题
    params.n_ubatch = 512; // 显式设置 ubatch size，避免默认值过小导致编码时断言失败
    params.n_gpu_layers = 0; // 强制使用CPU后端
    // 设置为空向量，不使用任何GPU设备，只使用CPU
    params.devices = std::vector<ggml_backend_dev_t>();

    // 禁用Metal后端，使用CPU后端
    setenv("GGML_METAL_PATH", "", 1);
    setenv("GGML_METAL", "0", 1);
    
    llama_backend_init();

    bctx->init_res = common_init_from_params(params);
    if (!bctx->init_res) {
        delete bctx;
        return nullptr;
    }

    bctx->model = bctx->init_res->model();
    bctx->ctx = bctx->init_res->context();
    if (bctx->model == nullptr || bctx->ctx == nullptr) {
        delete bctx;
        return nullptr;
    }

    bctx->chat_tmpls = common_chat_templates_init(bctx->model, "");
    if (!bctx->chat_tmpls) {
        delete bctx;
        return nullptr;
    }

    return bctx;
}

void llama_binding_free_model(void * ctx) {
    if (ctx) {
        auto * bctx = (LlamaBindingContext *) ctx;
        delete bctx;
    }
}

void llama_binding_free_result(char * result) {
    if (result) {
        free(result);
    }
}

char * llama_binding_chat(void * ctx, const char * messages_json, const char * stop_tokens, int n_predict, float temp, float top_p, int top_k, float repeat_penalty) {
    if (!ctx) {
        return nullptr;
    }
    auto * bctx = (LlamaBindingContext *) ctx;

    std::string prompt;
    std::vector<std::string> stops;
    if (!build_chat_prompt(bctx, messages_json, prompt, stops)) {
        return nullptr;
    }

    std::vector<std::string> extra_stops = split_unit_sep(stop_tokens);
    for (const auto & s : extra_stops) {
        if (!s.empty()) {
            stops.push_back(s);
        }
    }

    std::string out;
    if (!chat_generate(bctx, prompt, stops, n_predict, temp, top_p, top_k, repeat_penalty, 0, &out)) {
        return nullptr;
    }

    return strdup(out.c_str());
}

int llama_binding_chat_stream(void * ctx, const char * messages_json, const char * stop_tokens, int n_predict, float temp, float top_p, int top_k, float repeat_penalty, uintptr_t cb_handle) {
    if (!ctx) {
        return 1;
    }
    auto * bctx = (LlamaBindingContext *) ctx;

    std::string prompt;
    std::vector<std::string> stops;
    if (!build_chat_prompt(bctx, messages_json, prompt, stops)) {
        return 1;
    }

    std::vector<std::string> extra_stops = split_unit_sep(stop_tokens);
    for (const auto & s : extra_stops) {
        if (!s.empty()) {
            stops.push_back(s);
        }
    }

    if (!chat_generate(bctx, prompt, stops, n_predict, temp, top_p, top_k, repeat_penalty, cb_handle, nullptr)) {
        return 1;
    }

    return 0;
}

float* llama_binding_get_embedding(void* ctx, const char* text, int* out_dim) {
    if (!ctx || !text) {
        return nullptr;
    }
    auto* bctx = (LlamaBindingContext*) ctx;

    std::string prompt = text;
    std::vector<llama_token> tokens = common_tokenize(bctx->ctx, prompt, true, true);
    if (tokens.empty()) {
        return nullptr;
    }

    // 检查内存是否为空
    bool memory_empty = (llama_get_memory(bctx->ctx) == nullptr);
    
    // 获取批处理大小
    const uint32_t n_batch = llama_n_batch(bctx->ctx);
    
    // 对于空内存情况，使用更小的批处理大小，确保不超过n_ubatch
    uint32_t batch_size = memory_empty ? std::min(n_batch, (uint32_t)512) : n_batch;

    llama_memory_seq_rm(llama_get_memory(bctx->ctx), 0, -1, -1);

    llama_batch batch = llama_batch_init(batch_size, 0, 1);

    // 分块处理文本，避免一次性提交过多 token 导致内存不足
    for (size_t i = 0; i < tokens.size(); i += batch_size) {
        common_batch_clear(batch);
        
        size_t n_eval = tokens.size() - i;
        if (n_eval > batch_size) {
            n_eval = batch_size;
        }
        
        for (size_t j = 0; j < n_eval; j++) {
            common_batch_add(batch, tokens[i + j], (int32_t)(i + j), { 0 }, false);
        }
        
        // 只有最后一个 chunk 的最后一个 token 需要 logits
        if (i + n_eval == tokens.size()) {
            batch.logits[batch.n_tokens - 1] = true;
        }
        
        if (llama_decode(bctx->ctx, batch) != 0) {
            fprintf(stderr, "[llama_binding] Error: llama_decode failed during embedding processing at offset %zu, batch size: %u, n_eval: %zu\n", i, batch_size, n_eval);
            llama_batch_free(batch);
            return nullptr;
        }
    }

    int dim = llama_model_n_embd(bctx->model);
    if (out_dim) {
        *out_dim = dim;
    }

    float* embedding = (float*) malloc(dim * sizeof(float));
    if (!embedding) {
        llama_batch_free(batch);
        return nullptr;
    }

    // 解码成功，获取嵌入
    const float* data = llama_get_embeddings(bctx->ctx);
    if (data) {
        memcpy(embedding, data, dim * sizeof(float));
    } else {
        // 无法获取嵌入，使用默认值
        for (int i = 0; i < dim; i++) {
            embedding[i] = 0.0f;
        }
    }

    llama_batch_free(batch);
    return embedding;
}

void llama_binding_free_embedding(float* embedding) {
    if (embedding) {
        free(embedding);
    }
}

}
