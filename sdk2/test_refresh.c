#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include "libmpc.h"

// ANSI颜色代码
#define RESET   "\033[0m"
#define RED     "\033[31m"
#define GREEN   "\033[32m"
#define YELLOW  "\033[33m"
#define BLUE    "\033[34m"
#define MAGENTA "\033[35m"
#define CYAN    "\033[36m"
#define WHITE   "\033[37m"
#define BOLD    "\033[1m"

// 错误代码定义
#define MPC_SUCCESS 0
#define MPC_ERROR_INVALID_PARAM -1
#define MPC_ERROR_MEMORY -2
#define MPC_ERROR_CRYPTO -3
#define MPC_ERROR_NETWORK -4
#define MPC_ERROR_TIMEOUT -5

// 参与方数量和阈值
#define TOTAL_PARTIES 3
#define THRESHOLD 2
#define CURVE_SECP256K1 0

// 参与方密钥数据结构
typedef struct {
    int party_id;
    char* key_data;
    int key_len;
} PartyKeyData;

// 消息存储结构
typedef struct {
    char* data;
    int len;
} MessageData;

// 打印十六进制数据
void print_hex(const char* data, int len, int max_bytes) {
    int print_len = (len < max_bytes) ? len : max_bytes;
    for (int i = 0; i < print_len; i++) {
        printf("%02x", (unsigned char)data[i]);
    }
    if (len > max_bytes) {
        printf("...");
    }
}

// 从第一轮输出中提取消息并转换为Message数组格式
char* convert_round_to_messages(char** round_outputs, int* round_lens, int count, int target_party) {
    // 计算所需的缓冲区大小
    int total_size = 1000; // 初始大小
    for (int i = 0; i < count; i++) {
        total_size += round_lens[i] * 2; // 预留足够空间
    }
    
    char* result = malloc(total_size);
    strcpy(result, "[");
    
    int message_count = 0;
    
    // 遍历每个参与方的输出
    for (int i = 0; i < count; i++) {
        int from_party = i + 1;
        if (from_party == target_party) continue; // 跳过自己
        
        char* output = round_outputs[i];
        
        // 查找目标参与方的消息
        char target_key[10];
        sprintf(target_key, "\"%d\":", target_party);
        
        char* target_pos = strstr(output, target_key);
        if (!target_pos) continue;
        
        // 找到消息的开始位置
        char* msg_start = strchr(target_pos, '{');
        if (!msg_start) continue;
        
        // 找到消息的结束位置（匹配大括号）
        int brace_count = 0;
        char* msg_end = msg_start;
        do {
            if (*msg_end == '{') brace_count++;
            else if (*msg_end == '}') brace_count--;
            msg_end++;
        } while (brace_count > 0 && *msg_end);
        
        if (brace_count != 0) continue; // 格式错误
        
        // 添加逗号分隔符
        if (message_count > 0) {
            strcat(result, ",");
        }
        
        // 添加消息
        strncat(result, msg_start, msg_end - msg_start);
        message_count++;
    }
    
    strcat(result, "]");
    
    return result;
}

// 执行真实的密钥生成
int generate_real_keys(PartyKeyData* party_keys) {
    printf(BOLD CYAN "🔐 步骤1: 生成真实的密钥数据\n" RESET);
    printf("使用真实的MPC keygen协议生成密钥...\n\n");
    
    const int curve = 1; // secp256k1
    void* handles[TOTAL_PARTIES] = {NULL, NULL, NULL};
    char* round1_outputs[TOTAL_PARTIES] = {NULL, NULL, NULL};
    int round1_lens[TOTAL_PARTIES] = {0, 0, 0};
    char* round2_outputs[TOTAL_PARTIES] = {NULL, NULL, NULL};
    int round2_lens[TOTAL_PARTIES] = {0, 0, 0};
    
    // 初始化keygen会话
    printf(YELLOW "   🚀 初始化keygen会话\n" RESET);
    for (int i = 0; i < TOTAL_PARTIES; i++) {
        int party_id = i + 1;
        int ret = go_keygen_init(curve, party_id, THRESHOLD, TOTAL_PARTIES, &handles[i]);
        if (ret != 0) {
            printf(RED "   ❌ 参与方%d keygen初始化失败，错误码: %d\n" RESET, party_id, ret);
            return -1;
        }
        printf(GREEN "   ✅ 参与方%d keygen初始化成功\n" RESET, party_id);
    }
    
    // 执行keygen第一轮
     printf(YELLOW "\n   🔄 执行keygen第一轮\n" RESET);
     for (int i = 0; i < TOTAL_PARTIES; i++) {
         int ret = go_keygen_round1(handles[i], &round1_outputs[i], &round1_lens[i]);
         if (ret != 0) {
             printf(RED "   ❌ 参与方%d keygen第一轮失败，错误码: %d\n" RESET, i+1, ret);
             return -1;
         }
         printf(GREEN "   ✅ 参与方%d keygen第一轮完成，输出长度: %d\n" RESET, i+1, round1_lens[i]);
     }
     
     // 执行keygen第二轮
     printf(YELLOW "\n   🔄 执行keygen第二轮\n" RESET);
     for (int i = 0; i < TOTAL_PARTIES; i++) {
         int party_id = i + 1;
         
         // 为当前参与方转换消息
         char* messages_for_party = convert_round_to_messages(round1_outputs, round1_lens, TOTAL_PARTIES, party_id);
         
         int ret = go_keygen_round2(handles[i], messages_for_party, strlen(messages_for_party), 
                                    &round2_outputs[i], &round2_lens[i]);
         
         if (ret != 0) {
             printf(RED "   ❌ 参与方%d keygen第二轮失败，错误码: %d\n" RESET, party_id, ret);
             free(messages_for_party);
             return -1;
         }
         
         printf(GREEN "   ✅ 参与方%d keygen第二轮完成，输出长度: %d\n" RESET, party_id, round2_lens[i]);
         free(messages_for_party);
     }
     
     // 执行keygen第三轮并获取最终密钥
     printf(YELLOW "\n   🔄 执行keygen第三轮\n" RESET);
     for (int i = 0; i < TOTAL_PARTIES; i++) {
         int party_id = i + 1;
         
         // 为当前参与方转换第二轮消息
         char* messages_for_party = convert_round_to_messages(round2_outputs, round2_lens, TOTAL_PARTIES, party_id);
         
         int ret = go_keygen_round3(handles[i], messages_for_party, strlen(messages_for_party), 
                                    &party_keys[i].key_data, &party_keys[i].key_len);
         
         if (ret != 0) {
             printf(RED "   ❌ 参与方%d keygen第三轮失败，错误码: %d\n" RESET, party_id, ret);
             free(messages_for_party);
             return -1;
         }
         
         party_keys[i].party_id = party_id;
         printf(GREEN "   ✅ 参与方%d keygen第三轮完成，密钥长度: %d\n" RESET, party_id, party_keys[i].key_len);
         printf("      密钥预览: %.100s...\n", party_keys[i].key_data);
         free(messages_for_party);
     }
    
    // 清理keygen资源
    for (int i = 0; i < TOTAL_PARTIES; i++) {
        if (handles[i]) go_keygen_destroy(handles[i]);
        if (round1_outputs[i]) free(round1_outputs[i]);
        if (round2_outputs[i]) free(round2_outputs[i]);
    }
    
    printf(GREEN "\n   🎉 真实密钥生成完成！\n" RESET);
    return 0;
}

// 执行密钥刷新测试
int test_refresh() {
    printf(BOLD CYAN "🔄 开始MPC密钥刷新测试\n" RESET);
    printf("参数配置: %d个参与方, %d/%d阈值方案, SECP256K1曲线\n\n", 
           TOTAL_PARTIES, THRESHOLD, TOTAL_PARTIES);

    // 存储每个参与方的会话句柄
    void* handles[TOTAL_PARTIES];
    PartyKeyData party_keys[TOTAL_PARTIES];
    MessageData round1_messages[TOTAL_PARTIES];
    MessageData round2_messages[TOTAL_PARTIES];
    MessageData final_keys[TOTAL_PARTIES];

    // 初始化所有数据
    memset(handles, 0, sizeof(handles));
    memset(party_keys, 0, sizeof(party_keys));
    memset(round1_messages, 0, sizeof(round1_messages));
    memset(round2_messages, 0, sizeof(round2_messages));
    memset(final_keys, 0, sizeof(final_keys));

    // 步骤1: 生成真实的密钥数据（而不是mock数据）
    if (generate_real_keys(party_keys) != 0) {
        printf(RED "❌ 真实密钥生成失败\n" RESET);
        goto cleanup;
    }

    // 步骤2: 初始化refresh会话
    printf(BOLD YELLOW "\n🚀 步骤2: 初始化refresh会话\n" RESET);
    for (int i = 0; i < TOTAL_PARTIES; i++) {
        // devoteList参数 - 指定要刷新的参与方列表
        int devoteList[2] = {1, 2}; // 刷新参与方1和2
        int devoteCount = 2;
        
        int ret = go_refresh_init(
            CURVE_SECP256K1,           // 曲线类型
            party_keys[i].party_id,    // 参与方ID
            THRESHOLD,                 // 阈值
            devoteList,               // 要刷新的参与方列表
            devoteCount,              // 刷新参与方数量
            party_keys[i].key_data,   // 真实的密钥数据
            party_keys[i].key_len,    // 密钥数据长度
            &handles[i]               // 输出会话句柄
        );

        if (ret != MPC_SUCCESS) {
            printf(RED "   ❌ 参与方%d初始化失败，错误代码: %d\n" RESET, i + 1, ret);
            char* error_msg = mpc_get_error_string(ret);
            if (error_msg) {
                printf("      错误信息: %s\n", error_msg);
                mpc_string_free(error_msg);
            }
            goto cleanup;
        }
        
        printf(GREEN "   ✅ 参与方%d初始化成功\n" RESET, i + 1);
    }
    printf("\n");

    // 步骤3: 执行第一轮
    printf(BOLD YELLOW "🔄 步骤3: 执行第一轮refresh\n" RESET);
    for (int i = 0; i < TOTAL_PARTIES; i++) {
        char* out_data = NULL;
        int out_len = 0;
        
        int ret = go_refresh_round1(handles[i], &out_data, &out_len);
        
        if (ret != MPC_SUCCESS) {
            printf(RED "   ❌ 参与方%d第一轮失败，错误代码: %d\n" RESET, i + 1, ret);
            goto cleanup;
        }
        
        // 保存第一轮消息
        round1_messages[i].data = out_data;
        round1_messages[i].len = out_len;
        
        printf(GREEN "   ✅ 参与方%d第一轮完成，消息长度: %d\n" RESET, i + 1, out_len);
        printf("      消息预览: %.60s...\n", out_data ? out_data : "null");
    }
    printf("\n");

    // 步骤4: 执行第二轮
    printf(BOLD YELLOW "🔄 步骤4: 执行第二轮refresh\n" RESET);
    for (int i = 0; i < TOTAL_PARTIES; i++) {
        // 根据Go测试代码的消息路由模式：
        // 参与方i接收所有其他参与方发送给参与方i的消息
        char* aggregated_messages = malloc(20000);
        strcpy(aggregated_messages, "[");
        
        int first = 1;
        for (int j = 0; j < TOTAL_PARTIES; j++) {
            if (j != i) { // 从其他参与方的消息中选择
                char* msg_data = round1_messages[j].data;
                if (msg_data && strlen(msg_data) > 2) {
                    // 解析JSON对象，查找发送给参与方(i+1)的消息
                    // 格式: {"target_id":{"From":sender,"To":target,"Data":"..."}}
                    char target_key[10];
                    sprintf(target_key, "\"%d\":", i + 1);
                    
                    char* target_msg = strstr(msg_data, target_key);
                    if (target_msg) {
                        // 找到发送给当前参与方的消息
                        target_msg += strlen(target_key); // 跳过key部分
                        
                        // 找到消息对象的开始和结束
                        if (*target_msg == '{') {
                            int brace_count = 1;
                            char* msg_end = target_msg + 1;
                            while (*msg_end && brace_count > 0) {
                                if (*msg_end == '{') brace_count++;
                                else if (*msg_end == '}') brace_count--;
                                msg_end++;
                            }
                            
                            if (brace_count == 0) {
                                if (!first) {
                                    strcat(aggregated_messages, ",");
                                }
                                
                                // 提取消息对象
                                int msg_len = msg_end - target_msg;
                                char* extracted_msg = malloc(msg_len + 1);
                                strncpy(extracted_msg, target_msg, msg_len);
                                extracted_msg[msg_len] = '\0';
                                
                                strcat(aggregated_messages, extracted_msg);
                                free(extracted_msg);
                                first = 0;
                            }
                        }
                    }
                }
            }
        }
        strcat(aggregated_messages, "]");
        
        char* out_data = NULL;
        int out_len = 0;
        
        int ret = go_refresh_round2(handles[i], aggregated_messages, strlen(aggregated_messages), &out_data, &out_len);
        
        if (ret != MPC_SUCCESS) {
            printf(RED "   ❌ 参与方%d第二轮失败，错误代码: %d\n" RESET, i + 1, ret);
            printf("      输入消息长度: %d\n", (int)strlen(aggregated_messages));
            printf("      输入消息预览: %.200s...\n", aggregated_messages);
            free(aggregated_messages);
            goto cleanup;
        }
        
        // 保存第二轮消息
        round2_messages[i].data = out_data;
        round2_messages[i].len = out_len;
        
        printf(GREEN "   ✅ 参与方%d第二轮完成，消息长度: %d\n" RESET, i + 1, out_len);
        printf("      消息预览: %.60s...\n", out_data ? out_data : "null");
        
        // 调试：检查第二轮输出包含哪些目标
        printf("      调试: 参与方%d第二轮输出分析:\n", i + 1);
        for (int target = 1; target <= TOTAL_PARTIES; target++) {
            if (target != i + 1) {
                char target_key[10];
                sprintf(target_key, "\"%d\":", target);
                if (strstr(out_data, target_key)) {
                    printf("        包含发送给参与方%d的消息 ✅\n", target);
                } else {
                    printf("        缺少发送给参与方%d的消息 ❌\n", target);
                }
            }
        }
        
        free(aggregated_messages);
    }
    printf("\n");

    // 步骤5: 执行第三轮并生成新密钥
    printf(BOLD YELLOW "🔄 步骤5: 执行第三轮refresh并生成新密钥\n" RESET);
    for (int i = 0; i < TOTAL_PARTIES; i++) {
        // 根据Go测试代码的消息路由模式：
        // 参与方i接收所有其他参与方发送给参与方i的消息
        char* aggregated_messages = malloc(20000);
        strcpy(aggregated_messages, "[");
        
        int first = 1;
        int message_count = 0;
        for (int j = 0; j < TOTAL_PARTIES; j++) {
            if (j != i) { // 从其他参与方的消息中选择
                char* msg_data = round2_messages[j].data;
                if (msg_data && strlen(msg_data) > 2) {
                    // 解析JSON对象，查找发送给参与方(i+1)的消息
                    // 格式: {"target_id":{"From":sender,"To":target,"Data":"..."}}
                    char target_key[10];
                    sprintf(target_key, "\"%d\":" , i + 1);
                    
                    char* target_msg = strstr(msg_data, target_key);
                    if (target_msg) {
                        // 找到发送给当前参与方的消息
                        target_msg += strlen(target_key); // 跳过key部分
                        
                        // 找到消息对象的开始和结束
                        if (*target_msg == '{') {
                            int brace_count = 1;
                            char* msg_end = target_msg + 1;
                            while (*msg_end && brace_count > 0) {
                                if (*msg_end == '{') brace_count++;
                                else if (*msg_end == '}') brace_count--;
                                msg_end++;
                            }
                            
                            if (brace_count == 0) {
                                if (!first) {
                                    strcat(aggregated_messages, ",");
                                }
                                
                                // 提取消息对象
                                int msg_len = msg_end - target_msg;
                                char* extracted_msg = malloc(msg_len + 1);
                                strncpy(extracted_msg, target_msg, msg_len);
                                extracted_msg[msg_len] = '\0';
                                
                                strcat(aggregated_messages, extracted_msg);
                                free(extracted_msg);
                                first = 0;
                                message_count++;
                                
                                printf("      调试: 参与方%d从参与方%d提取消息长度: %d\n", i + 1, j + 1, msg_len);
                            }
                        }
                    } else {
                        printf("      调试: 参与方%d在参与方%d的消息中未找到目标key: %s\n", i + 1, j + 1, target_key);
                    }
                }
            }
        }
        strcat(aggregated_messages, "]");
        
        char* key_data = NULL;
        int key_len = 0;
        
        printf("      调试: 参与方%d第三轮期望消息数量: %d, 实际提取数量: %d\n", i + 1, TOTAL_PARTIES - 1, message_count);
        printf("      调试: 参与方%d第三轮输入消息长度: %d\n", i + 1, (int)strlen(aggregated_messages));
        printf("      调试: 输入消息预览: %.300s...\n", aggregated_messages);
        
        printf("      调试: 即将调用go_refresh_round3函数...\n");
        fflush(stdout);
        int ret = go_refresh_round3(handles[i], aggregated_messages, strlen(aggregated_messages), &key_data, &key_len);
        printf("      调试: go_refresh_round3函数返回，错误代码: %d\n", ret);
        fflush(stdout);
        
        if (ret != MPC_SUCCESS) {
            printf(RED "   ❌ 参与方%d第三轮失败，错误代码: %d\n" RESET, i + 1, ret);
            printf("      输入消息长度: %d\n", (int)strlen(aggregated_messages));
            printf("      输入消息预览: %.200s...\n", aggregated_messages);
            free(aggregated_messages);
            goto cleanup;
        }
        
        // 保存新密钥
        final_keys[i].data = key_data;
        final_keys[i].len = key_len;
        
        printf(GREEN "   ✅ 参与方%d第三轮完成，新密钥长度: %d\n" RESET, i + 1, key_len);
        
        free(aggregated_messages);
    }
    printf("\n");

    // 步骤6: 显示刷新后的密钥
    printf(BOLD GREEN "🎊 密钥刷新成功完成！\n\n" RESET);
    printf(BOLD CYAN "📋 刷新后的密钥分片:\n\n" RESET);
    
    for (int i = 0; i < TOTAL_PARTIES; i++) {
        printf(BOLD "参与方%d的新密钥分片:\n" RESET, i + 1);
        printf("   长度: %d\n", final_keys[i].len);
        
        if (final_keys[i].data && final_keys[i].len > 0) {
            printf("   内容预览: %.100s", final_keys[i].data);
            if (final_keys[i].len > 100) {
                printf("...");
            }
            printf("\n");
            
            printf("   十六进制 (前64字节): ");
            print_hex(final_keys[i].data, final_keys[i].len, 64);
            printf("\n");
        } else {
            printf("   " RED "无效的密钥数据\n" RESET);
        }
        printf("\n");
    }

    printf(BOLD GREEN "🎊 refresh测试完成！成功刷新了所有密钥分片！\n" RESET);

cleanup:
    // 清理资源
    printf(BOLD YELLOW "\n🧹 清理资源...\n" RESET);
    
    // 销毁会话
    for (int i = 0; i < TOTAL_PARTIES; i++) {
        if (handles[i]) {
            go_refresh_destroy(handles[i]);
        }
    }
    
    // 释放内存
    for (int i = 0; i < TOTAL_PARTIES; i++) {
        if (party_keys[i].key_data) {
            free(party_keys[i].key_data);
        }
        if (round1_messages[i].data) {
            free(round1_messages[i].data);
        }
        if (round2_messages[i].data) {
            free(round2_messages[i].data);
        }
        if (final_keys[i].data) {
            free(final_keys[i].data);
        }
    }
    
    printf(GREEN "✅ 资源清理完成\n" RESET);
    return MPC_SUCCESS;
}

int main() {
    printf(BOLD MAGENTA "=" RESET);
    for (int i = 0; i < 60; i++) printf("=");
    printf("\n");
    printf(BOLD MAGENTA "🔄 MPC密钥刷新(Refresh)测试程序\n" RESET);
    printf(BOLD MAGENTA "=" RESET);
    for (int i = 0; i < 60; i++) printf("=");
    printf("\n\n");

    int result = test_refresh();
    
    if (result == MPC_SUCCESS) {
        printf(BOLD GREEN "\n🎉 所有测试通过！\n" RESET);
        return 0;
    } else {
        printf(BOLD RED "\n❌ 测试失败！\n" RESET);
        return 1;
    }
}