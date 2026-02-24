#ifndef BINDING_H
#define BINDING_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

void* llama_binding_load_model(const char* model_path, int n_ctx, int n_threads, int n_gpu_layers);
char* llama_binding_chat(void* ctx, const char* messages_json, const char* stop_tokens, int n_predict, float temp, float top_p, int top_k, float repeat_penalty);
int llama_binding_chat_stream(void* ctx, const char* messages_json, const char* stop_tokens, int n_predict, float temp, float top_p, int top_k, float repeat_penalty, uintptr_t cb_handle);
void llama_binding_free_model(void* ctx);
void llama_binding_free_result(char* result);

int llama_binding_go_on_token(uintptr_t cb_handle, char* token_piece);

#ifdef __cplusplus
}
#endif

#endif
