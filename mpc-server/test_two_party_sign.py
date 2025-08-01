#!/usr/bin/env python3
# -*- coding: utf-8 -*-

"""
2方MPC签名测试脚本
演示enterprise服务器和mobile-app服务器之间的2方签名流程
"""

import requests
import json
import time

# API配置
ENTERPRISE_API = "http://localhost:8082/api/v1"
MOBILE_API = "http://localhost:8083/api/v1"  # 假设mobile-app服务器在8083端口

def test_two_party_keygen():
    """测试2方密钥生成"""
    print("=== 2方密钥生成测试 ===")
    
    # 在enterprise服务器上发起密钥生成
    keygen_data = {
        "participants": ["enterprise", "mobile-app"],
        "threshold": 2
    }
    
    response = requests.post(f"{ENTERPRISE_API}/keygen", json=keygen_data)
    if response.status_code == 200:
        result = response.json()
        print(f"密钥生成成功: {result}")
        return result['session_id']
    else:
        print(f"密钥生成失败: {response.text}")
        return None

def wait_for_keygen_completion(session_id, timeout=30):
    """等待密钥生成完成"""
    print(f"等待密钥生成完成，会话ID: {session_id}")
    
    start_time = time.time()
    while time.time() - start_time < timeout:
        response = requests.get(f"{ENTERPRISE_API}/sessions/{session_id}")
        if response.status_code == 200:
            session = response.json()
            status = session.get('status')
            print(f"密钥生成状态: {status}")
            
            if status == 'completed':
                print("密钥生成完成!")
                return True
            elif status == 'failed':
                print("密钥生成失败!")
                return False
        
        time.sleep(2)
    
    print("密钥生成超时!")
    return False

def test_two_party_sign(keygen_session_id):
    """测试2方签名"""
    print("=== 2方签名测试 ===")
    
    # 在enterprise服务器上发起签名
    sign_data = {
        "session_id": keygen_session_id,  # 使用密钥生成的会话ID
        "message": "Hello, Two-Party MPC Signature!",
        "partner": "mobile-app"  # 指定签名伙伴
    }
    
    response = requests.post(f"{ENTERPRISE_API}/sign/two-party", json=sign_data)
    if response.status_code == 200:
        result = response.json()
        print(f"2方签名发起成功: {result}")
        return result['session_id']
    else:
        print(f"2方签名发起失败: {response.text}")
        return None

def wait_for_sign_completion(session_id, timeout=60):
    """等待签名完成"""
    print(f"等待签名完成，会话ID: {session_id}")
    
    start_time = time.time()
    while time.time() - start_time < timeout:
        response = requests.get(f"{ENTERPRISE_API}/sessions/{session_id}")
        if response.status_code == 200:
            session = response.json()
            status = session.get('status')
            print(f"签名状态: {status}")
            
            if status == 'completed':
                print("签名完成!")
                signature = session.get('data', {}).get('signature')
                if signature:
                    print(f"签名结果: {signature}")
                return True
            elif status == 'failed':
                print("签名失败!")
                error = session.get('data', {}).get('error')
                if error:
                    print(f"错误信息: {error}")
                return False
        
        time.sleep(3)
    
    print("签名超时!")
    return False

def main():
    """主测试流程"""
    print("开始2方MPC签名测试...")
    
    # 步骤1: 密钥生成
    keygen_session_id = test_two_party_keygen()
    if not keygen_session_id:
        print("密钥生成失败，退出测试")
        return
    
    # 步骤2: 等待密钥生成完成
    if not wait_for_keygen_completion(keygen_session_id):
        print("密钥生成未完成，退出测试")
        return
    
    # 步骤3: 执行签名
    sign_session_id = test_two_party_sign(keygen_session_id)
    if not sign_session_id:
        print("签名发起失败，退出测试")
        return
    
    # 步骤4: 等待签名完成
    if wait_for_sign_completion(sign_session_id):
        print("=== 2方MPC签名测试成功完成! ===")
    else:
        print("=== 2方MPC签名测试失败! ===")

if __name__ == "__main__":
    main()