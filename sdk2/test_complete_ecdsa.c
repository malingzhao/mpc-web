#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "libmpc.h"

// 将字符串转换为十六进制字符串
char* string_to_hex(const char* input) {
    size_t input_len = strlen(input);
    char* hex_output = malloc(input_len * 2 + 1);
    if (!hex_output) return NULL;
    
    for (size_t i = 0; i < input_len; i++) {
        sprintf(hex_output + i * 2, "%02x", (unsigned char)input[i]);
    }
    hex_output[input_len * 2] = '\0';
    return hex_output;
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

// 从第二轮输出中提取消息并转换为Message数组格式
char* convert_round2_to_messages(char** round2_outputs, int* round2_lens, int count, int target_party) {
    // 计算所需的缓冲区大小
    int total_size = 1000; // 初始大小
    for (int i = 0; i < count; i++) {
        total_size += round2_lens[i] * 2; // 预留足够空间
    }
    
    char* result = malloc(total_size);
    strcpy(result, "[");
    
    int message_count = 0;
    
    // 遍历每个参与方的输出
    for (int i = 0; i < count; i++) {
        int from_party = i + 1;
        if (from_party == target_party) continue; // 跳过自己
        
        char* output = round2_outputs[i];
        
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
    printf("=== 完整ECDSA测试（DKG + Keygen + 签名）===\n");
    
    // 第一阶段：DKG密钥生成
    printf("\n第一阶段：DKG密钥生成\n");
    
    const int curve = 0; // secp256k1
    const int threshold = 2;
    const int total_parties = 3;
    
    void* handles[3] = {NULL, NULL, NULL};
    char* round1_outputs[3] = {NULL, NULL, NULL};
    int round1_lens[3] = {0, 0, 0};
    char* round2_outputs[3] = {NULL, NULL, NULL};
    int round2_lens[3] = {0, 0, 0};
    char* dkg_keys[3] = {NULL, NULL, NULL};
    int dkg_lens[3] = {0, 0, 0};
    
    // DKG初始化
    printf("1. DKG初始化...\n");
    for (int i = 0; i < 3; i++) {
        int party_id = i + 1;
        int result = go_keygen_init(curve, party_id, threshold, total_parties, &handles[i]);
        if (result != 0) {
            printf("❌ 参与方%d DKG初始化失败: %d\n", party_id, result);
            return 1;
        }
        printf("   ✅ 参与方%d DKG初始化成功\n", party_id);
    }
    
    // DKG第一轮
    printf("2. DKG第一轮...\n");
    for (int i = 0; i < 3; i++) {
        int result = go_keygen_round1(handles[i], &round1_outputs[i], &round1_lens[i]);
        if (result != 0) {
            printf("❌ 参与方%d DKG第一轮失败: %d\n", i+1, result);
            return 1;
        }
        printf("   ✅ 参与方%d DKG第一轮完成\n", i+1);
    }
    
    // DKG第二轮
    printf("3. DKG第二轮...\n");
    for (int i = 0; i < 3; i++) {
        int party_id = i + 1;
        char* messages_for_party = convert_round1_to_messages(round1_outputs, round1_lens, 3, party_id);
        
        int result = go_keygen_round2(handles[i], messages_for_party, strlen(messages_for_party), 
                                     &round2_outputs[i], &round2_lens[i]);
        
        if (result != 0) {
            printf("❌ 参与方%d DKG第二轮失败: %d\n", party_id, result);
            free(messages_for_party);
            return 1;
        }
        
        printf("   ✅ 参与方%d DKG第二轮完成\n", party_id);
        free(messages_for_party);
    }
    
    // DKG第三轮
    printf("4. DKG第三轮...\n");
    for (int i = 0; i < 3; i++) {
        int party_id = i + 1;
        char* messages_for_party = convert_round2_to_messages(round2_outputs, round2_lens, 3, party_id);
        
        int result = go_keygen_round3(handles[i], messages_for_party, strlen(messages_for_party), 
                                     &dkg_keys[i], &dkg_lens[i]);
        
        if (result != 0) {
            printf("❌ 参与方%d DKG第三轮失败: %d\n", party_id, result);
            free(messages_for_party);
            return 1;
        }
        
        printf("   ✅ 参与方%d DKG第三轮完成，密钥长度: %d\n", party_id, dkg_lens[i]);
        free(messages_for_party);
    }
    
    printf("✅ DKG密钥生成完成\n");
    
    // 第二阶段：ECDSA Keygen（P1和P2之间）
    printf("\n第二阶段：ECDSA Keygen（P1和P2之间）\n");
    
    // 使用参与方1作为P1，参与方2作为P2
    int p1_id = 1;
    int p2_id = 2;
    
    char* p1_sign_data = NULL;
    int p1_sign_data_len = 0;
    char* p1_message = NULL;
    int p1_message_len = 0;
    
    char* p2_sign_data = NULL;
    int p2_sign_data_len = 0;
    
    // 首先生成P2的预参数
    printf("1. 生成P2预参数...\n");
    char* p2_params = NULL;
    int p2_params_len = 0;
    int result = go_ecdsa_keygen_generate_p2_params(&p2_params, &p2_params_len);
    if (result != 0) {
        printf("❌ P2预参数生成失败: %d (%s)\n", result, mpc_get_error_string(result));
        return 1;
    }
    printf("   ✅ P2预参数生成成功，长度: %d\n", p2_params_len);
    
    // P1执行keygen
    printf("2. P1执行keygen...\n");
    result = go_ecdsa_keygen_p1(dkg_keys[0], dkg_lens[0], p2_id, 
                               p2_params, p2_params_len,
                               &p1_sign_data, &p1_sign_data_len,
                               &p1_message, &p1_message_len);
    if (result != 0) {
        printf("❌ P1 keygen失败: %d (%s)\n", result, mpc_get_error_string(result));
        return 1;
    }
    printf("   ✅ P1 keygen成功，签名数据长度: %d，消息长度: %d\n", 
           p1_sign_data_len, p1_message_len);
    
    // P2执行keygen
    printf("3. P2执行keygen...\n");
    result = go_ecdsa_keygen_p2(dkg_keys[1], dkg_lens[1], p1_id,
                               p1_message, p1_message_len,
                               p2_params, p2_params_len,
                               &p2_sign_data, &p2_sign_data_len);
    if (result != 0) {
        printf("❌ P2 keygen失败: %d (%s)\n", result, mpc_get_error_string(result));
        return 1;
    }
    printf("   ✅ P2 keygen成功，签名数据长度: %d\n", p2_sign_data_len);
    
    printf("✅ ECDSA Keygen完成\n");
    
    // 第三阶段：ECDSA签名
    printf("\n第三阶段：ECDSA签名\n");
    
    const char* message_to_sign = "Hello, ECDSA MPC!";
    printf("要签名的消息: \"%s\"\n", message_to_sign);
    
    // 将消息转换为十六进制格式
    char* hex_message = string_to_hex(message_to_sign);
    if (!hex_message) {
        printf("❌ 消息转换为十六进制失败\n");
        return 1;
    }
    printf("十六进制消息: %s\n", hex_message);
    
    // 初始化P1签名
    printf("1. 初始化P1签名...\n");
    void* p1_sign_handle = NULL;
    result = go_ecdsa_sign_init_p1_complex(p1_id, p2_id, 
                                          p1_sign_data, p1_sign_data_len,
                                          hex_message, strlen(hex_message),
                                          &p1_sign_handle);
    if (result != 0) {
        printf("❌ P1签名初始化失败: %d (%s)\n", result, mpc_get_error_string(result));
        free(hex_message);
        return 1;
    }
    printf("   ✅ P1签名初始化成功\n");
    
    // 初始化P2签名
    printf("2. 初始化P2签名...\n");
    void* p2_sign_handle = NULL;
    result = go_ecdsa_sign_init_p2_complex(p2_id, p1_id,
                                          p2_sign_data, p2_sign_data_len,
                                          hex_message, strlen(hex_message),
                                          &p2_sign_handle);
    if (result != 0) {
        printf("❌ P2签名初始化失败: %d (%s)\n", result, mpc_get_error_string(result));
        free(hex_message);
        return 1;
    }
    printf("   ✅ P2签名初始化成功\n");
    
    // P1 Step1: 生成承诺
    printf("3. P1 Step1: 生成承诺...\n");
    char* p1_commit_data = NULL;
    int p1_commit_len = 0;
    result = go_ecdsa_sign_step1(p1_sign_handle, &p1_commit_data, &p1_commit_len);
    if (result != 0) {
        printf("❌ P1 Step1失败: %d (%s)\n", result, mpc_get_error_string(result));
        return 1;
    }
    printf("   ✅ P1 Step1成功，承诺数据长度: %d\n", p1_commit_len);
    
    // P2 Step1: 处理承诺并生成证明
    printf("4. P2 Step1: 处理承诺并生成证明...\n");
    char* p2_proof_data = NULL;
    int p2_proof_len = 0;
    char* p2_r2_data = NULL;
    int p2_r2_len = 0;
    result = go_ecdsa_sign_p2_step1(p2_sign_handle, p1_commit_data, p1_commit_len,
                                   &p2_proof_data, &p2_proof_len,
                                   &p2_r2_data, &p2_r2_len);
    if (result != 0) {
        printf("❌ P2 Step1失败: %d (%s)\n", result, mpc_get_error_string(result));
        return 1;
    }
    printf("   ✅ P2 Step1成功，证明数据长度: %d，R2数据长度: %d\n", 
           p2_proof_len, p2_r2_len);
    
    // P1 Step2: 处理P2的证明
    printf("5. P1 Step2: 处理P2的证明...\n");
    char* p1_proof_data = NULL;
    int p1_proof_len = 0;
    char* p1_cmtd_data = NULL;
    int p1_cmtd_len = 0;
    result = go_ecdsa_sign_p1_step2(p1_sign_handle, 
                                   p2_proof_data, p2_proof_len,
                                   p2_r2_data, p2_r2_len,
                                   &p1_proof_data, &p1_proof_len,
                                   &p1_cmtd_data, &p1_cmtd_len);
    if (result != 0) {
        printf("❌ P1 Step2失败: %d (%s)\n", result, mpc_get_error_string(result));
        return 1;
    }
    printf("   ✅ P1 Step2成功，P1证明数据长度: %d，承诺D数据长度: %d\n", 
           p1_proof_len, p1_cmtd_len);
    
    // P2 Step2: 处理P1的证明
    printf("6. P2 Step2: 处理P1的证明...\n");
    char* p2_ek_data = NULL;
    int p2_ek_len = 0;
    char* p2_affine_proof_data = NULL;
    int p2_affine_proof_len = 0;
    result = go_ecdsa_sign_p2_step2(p2_sign_handle,
                                   p1_cmtd_data, p1_cmtd_len,
                                   p1_proof_data, p1_proof_len,
                                   &p2_ek_data, &p2_ek_len,
                                   &p2_affine_proof_data, &p2_affine_proof_len);
    if (result != 0) {
        printf("❌ P2 Step2失败: %d (%s)\n", result, mpc_get_error_string(result));
        return 1;
    }
    printf("   ✅ P2 Step2成功，EK数据长度: %d，仿射证明数据长度: %d\n", 
           p2_ek_len, p2_affine_proof_len);
    
    // P1 Step3: 生成最终签名
    printf("7. P1 Step3: 生成最终签名...\n");
    char* signature_r = NULL;
    int signature_r_len = 0;
    char* signature_s = NULL;
    int signature_s_len = 0;
    result = go_ecdsa_sign_p1_step3(p1_sign_handle,
                                   p2_ek_data, p2_ek_len,
                                   p2_affine_proof_data, p2_affine_proof_len,
                                   &signature_r, &signature_r_len,
                                   &signature_s, &signature_s_len);
    if (result != 0) {
        printf("❌ P1 Step3失败: %d (%s)\n", result, mpc_get_error_string(result));
        return 1;
    }
    printf("   ✅ P1 Step3成功，生成签名!\n");
    printf("   📝 签名R: %.*s\n", signature_r_len, signature_r);
    printf("   📝 签名S: %.*s\n", signature_s_len, signature_s);
    
    printf("✅ ECDSA签名完成\n");
    
    // 释放十六进制消息内存
    free(hex_message);
    
    // 清理资源
    printf("\n第四阶段：清理资源\n");
    
    // 清理DKG资源
    for (int i = 0; i < 3; i++) {
        if (handles[i]) {
            go_keygen_destroy(handles[i]);
        }
    }
    
    // 清理签名资源
    if (p1_sign_handle) {
        go_ecdsa_sign_destroy(p1_sign_handle);
    }
    if (p2_sign_handle) {
        go_ecdsa_sign_destroy(p2_sign_handle);
    }
    
    // 清理分配的字符串（由Go分配的内存）
    if (p1_sign_data) free(p1_sign_data);
    if (p1_message) free(p1_message);
    if (p2_sign_data) free(p2_sign_data);
    if (p1_commit_data) free(p1_commit_data);
    if (p2_proof_data) free(p2_proof_data);
    if (p2_r2_data) free(p2_r2_data);
    if (p1_proof_data) free(p1_proof_data);
    if (p1_cmtd_data) free(p1_cmtd_data);
    if (p2_ek_data) free(p2_ek_data);
    if (p2_affine_proof_data) free(p2_affine_proof_data);
    if (signature_r) free(signature_r);
    if (signature_s) free(signature_s);
    
    printf("✅ 所有资源已清理\n");
    
    printf("\n=== 测试完成 ===\n");
    printf("📋 测试总结：\n");
    printf("  ✅ DKG密钥生成：成功\n");
    printf("  ✅ ECDSA Keygen：成功\n");
    printf("  ✅ ECDSA签名：成功\n");
    printf("  ✅ 资源清理：成功\n");
    printf("\n🎉 完整的ECDSA MPC流程测试成功！\n");
    
    return 0;
}