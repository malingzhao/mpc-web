#!/usr/bin/env python3
"""
测试MPC服务器的sign和reshare功能
"""

import requests
import json
import time
import sys

# 服务器配置
SERVERS = {
    'enterprise': 'http://localhost:8082',
    'mobile-app': 'http://localhost:8083'
}

def test_keygen():
    """测试密钥生成"""
    print("=== 测试密钥生成 ===")
    
    # 向enterprise服务器发起密钥生成请求
    response = requests.post(f"{SERVERS['enterprise']}/api/keygen", json={
        "threshold": 2,
        "participants": ["enterprise", "mobile-app"]
    })
    
    if response.status_code != 200:
        print(f"❌ 密钥生成失败: {response.text}")
        return None
    
    result = response.json()
    session_id = result['session_id']
    print(f"✅ 密钥生成会话创建成功: {session_id}")
    
    # 等待密钥生成完成
    print("等待密钥生成完成...")
    for i in range(30):  # 最多等待30秒
        time.sleep(1)
        
        # 检查会话状态
        response = requests.get(f"{SERVERS['enterprise']}/api/sessions/{session_id}")
        if response.status_code == 200:
            session = response.json()
            status = session.get('status', 'unknown')
            print(f"密钥生成状态: {status}")
            
            if status == 'completed':
                print("✅ 密钥生成完成")
                return session_id
            elif status == 'failed':
                print("❌ 密钥生成失败")
                return None
    
    print("❌ 密钥生成超时")
    return None

def test_sign(keygen_session_id):
    """测试签名功能"""
    print("\n=== 测试签名功能 ===")
    
    if not keygen_session_id:
        print("❌ 需要先完成密钥生成")
        return False
    
    # 准备签名消息
    message = "Hello, MPC Threshold Signature!"
    
    # 向enterprise服务器发起签名请求
    response = requests.post(f"{SERVERS['enterprise']}/api/sign", json={
        "message": message,
        "keygen_session_id": keygen_session_id,
        "participants": ["enterprise", "mobile-app"]
    })
    
    if response.status_code != 200:
        print(f"❌ 签名请求失败: {response.text}")
        return False
    
    result = response.json()
    session_id = result['session_id']
    print(f"✅ 签名会话创建成功: {session_id}")
    
    # 等待签名完成
    print("等待签名完成...")
    for i in range(30):  # 最多等待30秒
        time.sleep(1)
        
        # 检查会话状态
        response = requests.get(f"{SERVERS['enterprise']}/api/sessions/{session_id}")
        if response.status_code == 200:
            session = response.json()
            status = session.get('status', 'unknown')
            print(f"签名状态: {status}")
            
            if status == 'completed':
                signature = session.get('data', {}).get('signature', '')
                print(f"✅ 签名完成")
                print(f"签名结果: {signature[:40]}...")
                return True
            elif status == 'failed':
                print("❌ 签名失败")
                return False
    
    print("❌ 签名超时")
    return False

def test_reshare(keygen_session_id):
    """测试重分享功能"""
    print("\n=== 测试重分享功能 ===")
    
    if not keygen_session_id:
        print("❌ 需要先完成密钥生成")
        return False
    
    # 向enterprise服务器发起重分享请求
    response = requests.post(f"{SERVERS['enterprise']}/api/reshare", json={
        "keygen_session_id": keygen_session_id,
        "threshold": 2,
        "participants": ["enterprise", "mobile-app"]
    })
    
    if response.status_code != 200:
        print(f"❌ 重分享请求失败: {response.text}")
        return False
    
    result = response.json()
    session_id = result['session_id']
    print(f"✅ 重分享会话创建成功: {session_id}")
    
    # 等待重分享完成
    print("等待重分享完成...")
    for i in range(30):  # 最多等待30秒
        time.sleep(1)
        
        # 检查会话状态
        response = requests.get(f"{SERVERS['enterprise']}/api/sessions/{session_id}")
        if response.status_code == 200:
            session = response.json()
            status = session.get('status', 'unknown')
            print(f"重分享状态: {status}")
            
            if status == 'completed':
                print("✅ 重分享完成")
                return True
            elif status == 'failed':
                print("❌ 重分享失败")
                return False
    
    print("❌ 重分享超时")
    return False

def main():
    """主测试函数"""
    print("开始测试MPC服务器的sign和reshare功能\n")
    
    # 检查服务器状态
    print("检查服务器状态...")
    for name, url in SERVERS.items():
        try:
            response = requests.get(f"{url}/api/info")
            if response.status_code == 200:
                info = response.json()
                print(f"✅ {name} 服务器运行正常 (端口: {info['port']})")
            else:
                print(f"❌ {name} 服务器响应异常")
                return
        except Exception as e:
            print(f"❌ 无法连接到 {name} 服务器: {e}")
            return
    
    print()
    
    # 1. 测试密钥生成
    keygen_session_id = test_keygen()
    
    # 2. 测试签名
    sign_success = test_sign(keygen_session_id)
    
    # 3. 测试重分享
    reshare_success = test_reshare(keygen_session_id)
    
    # 总结
    print("\n=== 测试总结 ===")
    print(f"密钥生成: {'✅ 成功' if keygen_session_id else '❌ 失败'}")
    print(f"签名功能: {'✅ 成功' if sign_success else '❌ 失败'}")
    print(f"重分享功能: {'✅ 成功' if reshare_success else '❌ 失败'}")
    
    if keygen_session_id and sign_success and reshare_success:
        print("\n🎉 所有测试通过！")
        return 0
    else:
        print("\n❌ 部分测试失败")
        return 1

if __name__ == "__main__":
    sys.exit(main())