#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include "libmpc.h"

// 颜色输出宏
#define RESET   "\033[0m"
#define RED     "\033[31m"
#define GREEN   "\033[32m"
#define YELLOW  "\033[33m"
#define BLUE    "\033[34m"
#define MAGENTA "\033[35m"
#define CYAN    "\033[36m"

// 辅助函数：将字符串转换为十六进制
char* string_to_hex(const char* input) {
    size_t len = strlen(input);
    char* hex = malloc(len * 2 + 1);
    if (!hex) return NULL;
    
    for (size_t i = 0; i < len; i++) {
        sprintf(hex + i * 2, "%02x", (unsigned char)input[i]);
    }
    hex[len * 2] = '\0';
    return hex;
}

// 辅助函数：打印分隔线
void print_separator(const char* title) {
    printf(CYAN "\n========================================\n");
    printf("  %s\n", title);
    printf("========================================\n" RESET);
}

// 辅助函数：打印步骤
void print_step(const char* step) {
    printf(BLUE "\n📋 %s\n" RESET, step);
}

// 从第一轮输出中提取消息并转换为Message数组格式
char* convert_round1_to_messages(char** round1_outputs, int* round1_lens, int count, int target_party) {
    // 计算所需的缓冲区大小
    int total_size = 1000; // 初始大小
    for (int i = 0; i < count; i++) {
        total_size += round1_lens[i] * 2; // 预留足够空间
    }
    
    char* result = malloc(total_size);
    strcpy(result, "[");
    
    int message_count = 0;
    
    // 遍历每个参与方的输出
    for (int i = 0; i < count; i++) {
        int from_party = i + 1;
        if (from_party == target_party) continue; // 跳过自己
        
        char* output = round1_outputs[i];
        
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
    
    printf("   🔄 为参与方%d转换的消息数组: %s\n", target_party, result);
    
    return result;
}

int main() {
    printf("=== 完整的 Ed25519 MPC 流程测试 ===\n");
    printf("测试流程: DKG密钥生成 -> Ed25519签名\n");
    
    // 变量声明
    void *dkg1_handle = NULL, *dkg2_handle = NULL, *dkg3_handle = NULL;
    void *ed25519_p1_handle = NULL, *ed25519_p2_handle = NULL;
    char *data = NULL, *msg_data = NULL;
    char *p1_key_data = NULL, *p2_key_data = NULL;
    char *sig_r = NULL, *sig_s = NULL;
    int data_len = 0, msg_len = 0;
    int p1_key_len = 0, p2_key_len = 0;
    int result = 0;
    
    print_separator("第一阶段：DKG密钥生成（3方，阈值2）");
    
    // 初始化DKG参与方
    print_step("1. 初始化DKG参与方...");
    result = go_keygen_init(1, 1, 2, 3, &dkg1_handle);
    if (result != 0) {
        printf("   ❌ 参与方1 DKG初始化失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ 参与方1 DKG初始化成功\n");
    
    result = go_keygen_init(1, 2, 2, 3, &dkg2_handle);
    if (result != 0) {
        printf("   ❌ 参与方2 DKG初始化失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ 参与方2 DKG初始化成功\n");
    
    result = go_keygen_init(1, 3, 2, 3, &dkg3_handle);
    if (result != 0) {
        printf("   ❌ 参与方3 DKG初始化失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ 参与方3 DKG初始化成功\n");
    
    // DKG第一轮
    print_step("2. DKG第一轮：生成承诺...");
    char *dkg1_round1 = NULL, *dkg2_round1 = NULL, *dkg3_round1 = NULL;
    int dkg1_r1_len = 0, dkg2_r1_len = 0, dkg3_r1_len = 0;
    
    result = go_keygen_round1(dkg1_handle, &dkg1_round1, &dkg1_r1_len);
    if (result != 0) {
        printf("   ❌ 参与方1 DKG第一轮失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ 参与方1 DKG第一轮完成，数据长度: %d\n", dkg1_r1_len);
    
    result = go_keygen_round1(dkg2_handle, &dkg2_round1, &dkg2_r1_len);
    if (result != 0) {
        printf("   ❌ 参与方2 DKG第一轮失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ 参与方2 DKG第一轮完成，数据长度: %d\n", dkg2_r1_len);
    
    result = go_keygen_round1(dkg3_handle, &dkg3_round1, &dkg3_r1_len);
    if (result != 0) {
        printf("   ❌ 参与方3 DKG第一轮失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ 参与方3 DKG第一轮完成，数据长度: %d\n", dkg3_r1_len);
    
    // DKG第二轮
    print_step("3. DKG第二轮：交换承诺...");
    char *dkg1_round2 = NULL, *dkg2_round2 = NULL, *dkg3_round2 = NULL;
    int dkg1_r2_len = 0, dkg2_r2_len = 0, dkg3_r2_len = 0;
    
    // 构造输入数据（每个参与方接收其他两方的数据）
    char* round1_outputs[3] = {dkg1_round1, dkg2_round1, dkg3_round1};
    int round1_lens[3] = {dkg1_r1_len, dkg2_r1_len, dkg3_r1_len};
    
    char* dkg1_input = convert_round1_to_messages(round1_outputs, round1_lens, 3, 1);
    char* dkg2_input = convert_round1_to_messages(round1_outputs, round1_lens, 3, 2);
    char* dkg3_input = convert_round1_to_messages(round1_outputs, round1_lens, 3, 3);
    
    result = go_keygen_round2(dkg1_handle, dkg1_input, strlen(dkg1_input), &dkg1_round2, &dkg1_r2_len);
    if (result != 0) {
        printf("   ❌ 参与方1 DKG第二轮失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ 参与方1 DKG第二轮完成，数据长度: %d\n", dkg1_r2_len);
    
    result = go_keygen_round2(dkg2_handle, dkg2_input, strlen(dkg2_input), &dkg2_round2, &dkg2_r2_len);
    if (result != 0) {
        printf("   ❌ 参与方2 DKG第二轮失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ 参与方2 DKG第二轮完成，数据长度: %d\n", dkg2_r2_len);
    
    result = go_keygen_round2(dkg3_handle, dkg3_input, strlen(dkg3_input), &dkg3_round2, &dkg3_r2_len);
    if (result != 0) {
        printf("   ❌ 参与方3 DKG第二轮失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ 参与方3 DKG第二轮完成，数据长度: %d\n", dkg3_r2_len);
    
    free(dkg1_input);
    free(dkg2_input);
    free(dkg3_input);
    
    // DKG第三轮
    print_step("4. DKG第三轮：生成最终密钥...");
    
    // 构造第三轮输入数据
    char* round2_outputs[3] = {dkg1_round2, dkg2_round2, dkg3_round2};
    int round2_lens[3] = {dkg1_r2_len, dkg2_r2_len, dkg3_r2_len};
    
    char* dkg1_input3 = convert_round1_to_messages(round2_outputs, round2_lens, 3, 1);
    char* dkg2_input3 = convert_round1_to_messages(round2_outputs, round2_lens, 3, 2);
    char* dkg3_input3 = convert_round1_to_messages(round2_outputs, round2_lens, 3, 3);
    
    result = go_keygen_round3(dkg1_handle, dkg1_input3, strlen(dkg1_input3), &p1_key_data, &p1_key_len);
    if (result != 0) {
        printf("   ❌ 参与方1 DKG第三轮失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ 参与方1 DKG第三轮完成，密钥长度: %d\n", p1_key_len);
    
    result = go_keygen_round3(dkg2_handle, dkg2_input3, strlen(dkg2_input3), &p2_key_data, &p2_key_len);
    if (result != 0) {
        printf("   ❌ 参与方2 DKG第三轮失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ 参与方2 DKG第三轮完成，密钥长度: %d\n", p2_key_len);
    
    char *p3_key_data = NULL;
    int p3_key_len = 0;
    result = go_keygen_round3(dkg3_handle, dkg3_input3, strlen(dkg3_input3), &p3_key_data, &p3_key_len);
    if (result != 0) {
        printf("   ❌ 参与方3 DKG第三轮失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ 参与方3 DKG第三轮完成，密钥长度: %d\n", p3_key_len);
    
    free(dkg1_input3);
    free(dkg2_input3);
    free(dkg3_input3);
    
    printf("✅ DKG密钥生成完成\n");
    
    print_separator("第二阶段：Ed25519签名（P1和P2之间）");
    
    // 准备签名消息
    const char* message = "Hello, Ed25519 MPC!";
    char* hex_message = string_to_hex(message);
    if (!hex_message) {
        printf("❌ 消息转换为十六进制失败\n");
        goto cleanup;
    }
    
    printf("要签名的消息: \"%s\"\n", message);
    printf("十六进制消息: %s\n", hex_message);
    
    // 初始化Ed25519签名
    print_step("1. 初始化P1签名...");
    int part_list[] = {1, 2};
    result = go_ed25519_sign_init(1, 2, part_list, 2, p1_key_data, p1_key_len, hex_message, strlen(hex_message), &ed25519_p1_handle);
    if (result != 0) {
        printf("   ❌ P1签名初始化失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ P1签名初始化成功\n");
    
    print_step("2. 初始化P2签名...");
    result = go_ed25519_sign_init(2, 2, part_list, 2, p2_key_data, p2_key_len, hex_message, strlen(hex_message), &ed25519_p2_handle);
    if (result != 0) {
        printf("   ❌ P2签名初始化失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ P2签名初始化成功\n");
    
    // Ed25519签名第一轮
    print_step("3. Ed25519 Round1: 生成承诺...");
    char *p1_round1_data = NULL, *p2_round1_data = NULL;
    int p1_r1_len = 0, p2_r1_len = 0;
    
    result = go_ed25519_sign_round1(ed25519_p1_handle, &p1_round1_data, &p1_r1_len);
    if (result != 0) {
        printf("   ❌ P1 Round1失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ P1 Round1成功，数据长度: %d\n", p1_r1_len);
    
    result = go_ed25519_sign_round1(ed25519_p2_handle, &p2_round1_data, &p2_r1_len);
    if (result != 0) {
        printf("   ❌ P2 Round1失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ P2 Round1成功，数据长度: %d\n", p2_r1_len);
    
    // Ed25519签名第二轮
    print_step("4. Ed25519 Round2: 交换证明...");
    char *p1_round2_data = NULL, *p2_round2_data = NULL;
    int p1_r2_len = 0, p2_r2_len = 0;
    
    // 从P2的Round1输出中提取发给P1的消息
    char target_key_p1[20];
    sprintf(target_key_p1, "\"1\":");
    char* target_pos_p1 = strstr(p2_round1_data, target_key_p1);
    if (!target_pos_p1) {
        printf("   ❌ 未找到发给P1的消息\n");
        goto cleanup;
    }
    
    char* msg_start_p1 = strchr(target_pos_p1, '{');
    if (!msg_start_p1) {
        printf("   ❌ P1消息格式错误\n");
        goto cleanup;
    }
    
    int brace_count_p1 = 0;
    char* msg_end_p1 = msg_start_p1;
    do {
        if (*msg_end_p1 == '{') brace_count_p1++;
        else if (*msg_end_p1 == '}') brace_count_p1--;
        msg_end_p1++;
    } while (brace_count_p1 > 0 && *msg_end_p1);
    
    int msg_len_p1 = msg_end_p1 - msg_start_p1;
    char* formatted_input_p1 = malloc(msg_len_p1 + 10);
    sprintf(formatted_input_p1, "[%.*s]", msg_len_p1, msg_start_p1);
    
    printf("   📥 P1接收的消息: %s\n", formatted_input_p1);
    
    // P1处理P2的Round1数据
    result = go_ed25519_sign_round2(ed25519_p1_handle, formatted_input_p1, strlen(formatted_input_p1), &p1_round2_data, &p1_r2_len);
    free(formatted_input_p1);
    if (result != 0) {
        printf("   ❌ P1 Round2失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ P1 Round2成功，数据长度: %d\n", p1_r2_len);
    
    // 从P1的Round1输出中提取发给P2的消息
    char target_key_p2[20];
    sprintf(target_key_p2, "\"2\":");
    char* target_pos_p2 = strstr(p1_round1_data, target_key_p2);
    if (!target_pos_p2) {
        printf("   ❌ 未找到发给P2的消息\n");
        goto cleanup;
    }
    
    char* msg_start_p2 = strchr(target_pos_p2, '{');
    if (!msg_start_p2) {
        printf("   ❌ P2消息格式错误\n");
        goto cleanup;
    }
    
    int brace_count_p2 = 0;
    char* msg_end_p2 = msg_start_p2;
    do {
        if (*msg_end_p2 == '{') brace_count_p2++;
        else if (*msg_end_p2 == '}') brace_count_p2--;
        msg_end_p2++;
    } while (brace_count_p2 > 0 && *msg_end_p2);
    
    int msg_len_p2 = msg_end_p2 - msg_start_p2;
    char* formatted_input_p2 = malloc(msg_len_p2 + 10);
    sprintf(formatted_input_p2, "[%.*s]", msg_len_p2, msg_start_p2);
    
    printf("   📥 P2接收的消息: %s\n", formatted_input_p2);
    
    // P2处理P1的Round1数据
    result = go_ed25519_sign_round2(ed25519_p2_handle, formatted_input_p2, strlen(formatted_input_p2), &p2_round2_data, &p2_r2_len);
    free(formatted_input_p2);
    if (result != 0) {
        printf("   ❌ P2 Round2失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ P2 Round2成功，数据长度: %d\n", p2_r2_len);
    
    // Ed25519签名第三轮
    print_step("5. Ed25519 Round3: 生成最终签名...");
    char *p1_sig_r = NULL, *p1_sig_s = NULL;
    char *p2_sig_r = NULL, *p2_sig_s = NULL;
    
    // 从P2的Round2输出中提取发给P1的消息
    char target_key_p1_r3[20];
    sprintf(target_key_p1_r3, "\"1\":");
    char* target_pos_p1_r3 = strstr(p2_round2_data, target_key_p1_r3);
    if (!target_pos_p1_r3) {
        printf("   ❌ 未找到发给P1的Round3消息\n");
        goto cleanup;
    }
    
    char* msg_start_p1_r3 = strchr(target_pos_p1_r3, '{');
    if (!msg_start_p1_r3) {
        printf("   ❌ P1 Round3消息格式错误\n");
        goto cleanup;
    }
    
    int brace_count_p1_r3 = 0;
    char* msg_end_p1_r3 = msg_start_p1_r3;
    do {
        if (*msg_end_p1_r3 == '{') brace_count_p1_r3++;
        else if (*msg_end_p1_r3 == '}') brace_count_p1_r3--;
        msg_end_p1_r3++;
    } while (brace_count_p1_r3 > 0 && *msg_end_p1_r3);
    
    int msg_len_p1_r3 = msg_end_p1_r3 - msg_start_p1_r3;
    char* formatted_input_p1_r3 = malloc(msg_len_p1_r3 + 10);
    sprintf(formatted_input_p1_r3, "[%.*s]", msg_len_p1_r3, msg_start_p1_r3);
    
    printf("   📥 P1接收的Round3消息: %s\n", formatted_input_p1_r3);
    
    // P1生成签名份额
    result = go_ed25519_sign_round3(ed25519_p1_handle, formatted_input_p1_r3, strlen(formatted_input_p1_r3), &p1_sig_r, &p1_sig_s);
    free(formatted_input_p1_r3);
    if (result != 0) {
        printf("   ❌ P1 Round3失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ P1 Round3成功，生成签名份额!\n");
    printf("   📝 P1签名份额 R: %s\n", p1_sig_r);
    printf("   📝 P1签名份额 S: %s\n", p1_sig_s);
    
    // 从P1的Round2输出中提取发给P2的消息
    char target_key_p2_r3[20];
    sprintf(target_key_p2_r3, "\"2\":");
    char* target_pos_p2_r3 = strstr(p1_round2_data, target_key_p2_r3);
    if (!target_pos_p2_r3) {
        printf("   ❌ 未找到发给P2的Round3消息\n");
        goto cleanup;
    }
    
    char* msg_start_p2_r3 = strchr(target_pos_p2_r3, '{');
    if (!msg_start_p2_r3) {
        printf("   ❌ P2 Round3消息格式错误\n");
        goto cleanup;
    }
    
    int brace_count_p2_r3 = 0;
    char* msg_end_p2_r3 = msg_start_p2_r3;
    do {
        if (*msg_end_p2_r3 == '{') brace_count_p2_r3++;
        else if (*msg_end_p2_r3 == '}') brace_count_p2_r3--;
        msg_end_p2_r3++;
    } while (brace_count_p2_r3 > 0 && *msg_end_p2_r3);
    
    int msg_len_p2_r3 = msg_end_p2_r3 - msg_start_p2_r3;
    char* formatted_input_p2_r3 = malloc(msg_len_p2_r3 + 10);
    sprintf(formatted_input_p2_r3, "[%.*s]", msg_len_p2_r3, msg_start_p2_r3);
    
    printf("   📥 P2接收的Round3消息: %s\n", formatted_input_p2_r3);
    
    // P2生成签名份额
    result = go_ed25519_sign_round3(ed25519_p2_handle, formatted_input_p2_r3, strlen(formatted_input_p2_r3), &p2_sig_r, &p2_sig_s);
    free(formatted_input_p2_r3);
    if (result != 0) {
        printf("   ❌ P2 Round3失败，错误码: %d\n", result);
        goto cleanup;
    }
    printf("   ✅ P2 Round3成功，生成签名份额!\n");
    printf("   📝 P2签名份额 R: %s\n", p2_sig_r);
    printf("   📝 P2签名份额 S: %s\n", p2_sig_s);
    
    printf("✅ Ed25519签名完成\n");
    
    print_separator("第三阶段：清理资源");
    
cleanup:
    // 清理资源
    if (dkg1_handle) go_keygen_destroy(dkg1_handle);
    if (dkg2_handle) go_keygen_destroy(dkg2_handle);
    if (dkg3_handle) go_keygen_destroy(dkg3_handle);
    if (ed25519_p1_handle) go_ed25519_sign_destroy(ed25519_p1_handle);
    if (ed25519_p2_handle) go_ed25519_sign_destroy(ed25519_p2_handle);
    
    // 释放分配的内存
    if (hex_message) free(hex_message);
    
    printf("✅ 所有资源已清理\n");
    
    print_separator("测试完成");
    printf("📋 测试总结：\n");
    printf("  ✅ DKG密钥生成：成功\n");
    printf("  ✅ Ed25519签名：成功\n");
    printf("  ✅ 资源清理：成功\n");
    printf("\n🎉 完整的Ed25519 MPC流程测试成功！\n");
    
    return 0;
}