#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
测试MPC签名功能的脚本
"""

import requests
import json
import time
import hashlib
import uuid

# 企业服务器的API端点
ENTERPRISE_API = "http://localhost:8082/api/v1"

def test_keygen():
    """测试密钥生成"""
    print("开始测试密钥生成...")
    
    session_id = str(uuid.uuid4())
    response = requests.post(f"{ENTERPRISE_API}/keygen", json={
        "session_id": session_id,
        "threshold": 2,
        "participants": ["enterprise", "third-party"]
    })
    
    if response.status_code == 200:
        result = response.json()
        print(f"密钥生成成功: {result}")
        return result.get("session_id", session_id)
    else:
        print(f"密钥生成失败: {response.status_code} - {response.text}")
        return None

def test_sign(keygen_session_id, message):
    """测试签名"""
    print(f"开始测试签名，使用keygen会话ID: {keygen_session_id}")
    
    # 计算消息哈希
    message_hash = hashlib.sha256(message.encode()).hexdigest()
    
    response = requests.post(f"{ENTERPRISE_API}/sign", json={
        "session_id": keygen_session_id,
        "message": message_hash,
        "signers": ["enterprise", "third-party"]
    })
    
    if response.status_code == 200:
        result = response.json()
        print(f"签名成功: {result}")
        return result
    else:
        print(f"签名失败: {response.status_code} - {response.text}")
        return None

def main():
    print("=== MPC签名功能测试 ===")
    
    # 测试密钥生成
    keygen_session_id = test_keygen()
    if not keygen_session_id:
        print("密钥生成失败，退出测试")
        return
    
    # 等待密钥生成完成
    print("等待密钥生成完成...")
    time.sleep(10)
    
    # 检查keygen会话状态
    print("检查keygen会话状态...")
    status_response = requests.get(f"{ENTERPRISE_API}/sessions/{keygen_session_id}")
    if status_response.status_code == 200:
        status_data = status_response.json()
        print(f"Keygen会话状态: {status_data['status']}")
        if status_data['status'] != 'completed':
            print("Keygen会话尚未完成，等待更长时间...")
            time.sleep(10)
    
    # 测试签名
    test_message = "Hello, MPC Signature!"
    signature = test_sign(keygen_session_id, test_message)
    
    if signature:
        print("=== 测试完成 ===")
        print(f"消息: {test_message}")
        print(f"签名会话: {signature}")
        
        # 检查签名会话状态
        sign_session_id = signature.get("session_id")
        if sign_session_id:
            time.sleep(5)
            sign_status_response = requests.get(f"{ENTERPRISE_API}/sessions/{sign_session_id}")
            if sign_status_response.status_code == 200:
                sign_status_data = sign_status_response.json()
                print(f"签名会话状态: {sign_status_data['status']}")
                if 'signature' in sign_status_data.get('data', {}):
                    print(f"最终签名: {sign_status_data['data']['signature']}")
    else:
        print("签名测试失败")

if __name__ == "__main__":
    main()