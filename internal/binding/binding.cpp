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
        return false;
    }

    std::vector<llama_token> tokens_list = common_tokenize(bctx->ctx, prompt, true, true);
    const uint32_t n_ctx = llama_n_ctx(bctx->ctx);
    if (n_ctx > 0 && tokens_list.size() > (size_t) n_ctx) {
        std::vector<llama_token> trimmed;
        trimmed.reserve(n_ctx);
        trimmed.push_back(tokens_list.front());
        if (n_ctx > 1) {
            trimmed.insert(trimmed.end(), tokens_list.end() - (n_ctx - 1), tokens_list.end());
        }
        tokens_list.swap(trimmed);
    }

    llama_memory_seq_rm(llama_get_memory(bctx->ctx), 0, -1, -1);

    size_t batch_cap = std::max<size_t>(512, tokens_list.size());
    if (batch_cap > (size_t) std::numeric_limits<int>::max()) {
        common_sampler_free(sampler);
        return false;
    }

    llama_batch batch = llama_batch_init((int) batch_cap, 0, 1);
    common_batch_clear(batch);
    for (size_t i = 0; i < tokens_list.size(); i++) {
        common_batch_add(batch, tokens_list[i], i, { 0 }, false);
    }
    batch.logits[batch.n_tokens - 1] = true;

    if (llama_decode(bctx->ctx, batch) != 0) {
        common_sampler_free(sampler);
        llama_batch_free(batch);
        return false;
    }

    const llama_vocab * vocab = llama_model_get_vocab(bctx->model);

    std::string result;
    size_t sent_len = 0;

    int n_cur = batch.n_tokens;
    while (n_cur <= n_predict || n_predict < 0) {
        const llama_token new_token_id = common_sampler_sample(sampler, bctx->ctx, -1);
        common_sampler_accept(sampler, new_token_id, true);

        if (llama_vocab_is_eog(vocab, new_token_id)) {
            break;
        }

        const std::string piece = common_token_to_piece(bctx->ctx, new_token_id, false);
        if (!piece.empty()) {
            result.append(piece);
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
                const int keep_going = llama_binding_go_on_token(cb_handle, const_cast<char *>(delta.c_str()));
                if (keep_going == 0) {
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
    params.n_gpu_layers = n_gpu_layers;

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

}
