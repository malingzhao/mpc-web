#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// 颜色输出宏
#define RESET   "\033[0m"
#define RED     "\033[31m"
#define GREEN   "\033[32m"
#define YELLOW  "\033[33m"
#define BLUE    "\033[34m"
#define MAGENTA "\033[35m"
#define CYAN    "\033[36m"

// 声明外部Go函数
extern int go_keygen_init(int curve, int party_id, int threshold, int total_parties, void** handle);
extern int go_keygen_round1(void* handle, char** out_data, int* out_len);
extern int go_keygen_round2(void* handle, const char* in_data, int in_len, char** out_data, int* out_len);
extern int go_keygen_round3(void* handle, const char* in_data, int in_len, char** key_data, int* key_len);
extern void go_keygen_destroy(void* handle);

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

int test_corrected_keygen() {
    printf(CYAN "🔐 MPC密钥生成修正测试程序\n" RESET);
    printf("目标: 使用正确的消息格式完成三轮密钥生成\n");
    printf("========================================\n\n");
    
    const int curve = 0; // secp256k1
    const int threshold = 2;
    const int total_parties = 3;
    
    void* handles[3] = {NULL, NULL, NULL};
    char* round1_outputs[3] = {NULL, NULL, NULL};
    int round1_lens[3] = {0, 0, 0};
    char* round2_outputs[3] = {NULL, NULL, NULL};
    int round2_lens[3] = {0, 0, 0};
    char* final_keys[3] = {NULL, NULL, NULL};
    int final_lens[3] = {0, 0, 0};
    
    // 第一步：初始化参与方
    printf(BLUE "📋 第一步：初始化参与方\n" RESET);
    for (int i = 0; i < 3; i++) {
        int party_id = i + 1;
        int ret = go_keygen_init(curve, party_id, threshold, total_parties, &handles[i]);
        if (ret != 0) {
            printf(RED "❌ 参与方%d初始化失败，错误码: %d\n" RESET, party_id, ret);
            return -1;
        }
        printf("   ✅ 参与方%d初始化成功\n", party_id);
    }
    
    // 第二步：执行第一轮
    printf(BLUE "\n📋 第二步：执行第一轮密钥生成\n" RESET);
    for (int i = 0; i < 3; i++) {
        int ret = go_keygen_round1(handles[i], &round1_outputs[i], &round1_lens[i]);
        if (ret != 0) {
            printf(RED "❌ 参与方%d第一轮失败，错误码: %d\n" RESET, i+1, ret);
            return -1;
        }
        printf("   ✅ 参与方%d第一轮完成，输出长度: %d\n", i+1, round1_lens[i]);
    }
    
    // 第三步：转换消息格式并执行第二轮
    printf(BLUE "\n📋 第三步：转换消息格式并执行第二轮\n" RESET);
    for (int i = 0; i < 3; i++) {
        int party_id = i + 1;
        
        // 为当前参与方转换消息
        char* messages_for_party = convert_round1_to_messages(round1_outputs, round1_lens, 3, party_id);
        
        int ret = go_keygen_round2(handles[i], messages_for_party, strlen(messages_for_party), 
                                   &round2_outputs[i], &round2_lens[i]);
        
        if (ret != 0) {
            printf(RED "❌ 参与方%d第二轮失败，错误码: %d\n" RESET, party_id, ret);
            free(messages_for_party);
            return -1;
        }
        
        printf("   ✅ 参与方%d第二轮完成，输出长度: %d\n", party_id, round2_lens[i]);
        free(messages_for_party);
    }
    
    // 第四步：执行第三轮
    printf(BLUE "\n📋 第四步：执行第三轮密钥生成\n" RESET);
    for (int i = 0; i < 3; i++) {
        int party_id = i + 1;
        
        // 为当前参与方转换第二轮消息
        char* messages_for_party = convert_round1_to_messages(round2_outputs, round2_lens, 3, party_id);
        
        int ret = go_keygen_round3(handles[i], messages_for_party, strlen(messages_for_party), 
                                   &final_keys[i], &final_lens[i]);
        
        if (ret != 0) {
            printf(RED "❌ 参与方%d第三轮失败，错误码: %d\n" RESET, party_id, ret);
            free(messages_for_party);
            return -1;
        }
        
        printf("   ✅ 参与方%d第三轮完成，密钥长度: %d\n", party_id, final_lens[i]);
        free(messages_for_party);
    }
    
    // 第五步：显示最终结果
    printf(GREEN "\n🎊 密钥生成成功完成！\n" RESET);
    printf(YELLOW "\n📋 最终私钥分片:\n" RESET);
    
    for (int i = 0; i < 3; i++) {
        printf(MAGENTA "\n参与方%d的私钥分片:\n" RESET, i+1);
        printf("   长度: %d\n", final_lens[i]);
        printf("   内容预览: %.200s%s\n", final_keys[i], final_lens[i] > 200 ? "..." : "");
        
        // 显示十六进制格式
        printf("   十六进制 (前64字节): ");
        for (int j = 0; j < (final_lens[i] < 64 ? final_lens[i] : 64); j++) {
            printf("%02x", (unsigned char)final_keys[i][j]);
        }
        if (final_lens[i] > 64) printf("...");
        printf("\n");
    }
    
    // 清理资源
    for (int i = 0; i < 3; i++) {
        if (handles[i]) go_keygen_destroy(handles[i]);
        if (round1_outputs[i]) free(round1_outputs[i]);
        if (round2_outputs[i]) free(round2_outputs[i]);
        if (final_keys[i]) free(final_keys[i]);
    }
    
    return 0;
}

int main() {
    int result = test_corrected_keygen();
    
    if (result == 0) {
        printf(GREEN "\n🎊 测试完成！成功生成了完整的私钥分片！\n" RESET);
        return 0;
    } else {
        printf(RED "\n💥 测试失败！\n" RESET);
        return 1;
    }
}