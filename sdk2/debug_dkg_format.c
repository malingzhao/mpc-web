#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "libmpc.h"

int main() {
    printf("=== DKG消息格式调试 ===\n");
    
    const int curve = 0; // secp256k1
    const int threshold = 2;
    const int total_parties = 3;
    
    void* handles[3] = {NULL, NULL, NULL};
    char* round1_outputs[3] = {NULL, NULL, NULL};
    int round1_lens[3] = {0, 0, 0};
    
    // 初始化所有参与方
    printf("1. 初始化参与方...\n");
    for (int i = 0; i < 3; i++) {
        int party_id = i + 1;
        int result = go_keygen_init(curve, party_id, threshold, total_parties, &handles[i]);
        if (result != 0) {
            printf("参与方%d初始化失败: %d\n", party_id, result);
            return 1;
        }
        printf("   参与方%d初始化成功\n", party_id);
    }
    
    // 执行第一轮
    printf("2. 执行第一轮...\n");
    for (int i = 0; i < 3; i++) {
        int result = go_keygen_round1(handles[i], &round1_outputs[i], &round1_lens[i]);
        if (result != 0) {
            printf("参与方%d第一轮失败: %d\n", i+1, result);
            return 1;
        }
        printf("   参与方%d第一轮完成，输出长度: %d\n", i+1, round1_lens[i]);
        
        // 打印前200个字符
        printf("   输出内容: %.200s...\n", round1_outputs[i]);
    }
    
    // 清理资源
    for (int i = 0; i < 3; i++) {
        go_keygen_destroy(handles[i]);
    }
    
    return 0;
}