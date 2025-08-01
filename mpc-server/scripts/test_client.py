#!/usr/bin/env python3
"""
MPC Server 测试客户端
用于测试三个MPC服务器之间的密钥生成、重分享和签名功能
"""

import requests
import json
import time
import sys
from typing import Dict, List, Optional

class MPCClient:
    def __init__(self, base_url: str, server_name: str):
        self.base_url = base_url
        self.server_name = server_name
        
    def get_server_info(self) -> Dict:
        """获取服务器信息"""
        try:
            response = requests.get(f"{self.base_url}/api/v1/info")
            response.raise_for_status()
            return response.json()
        except Exception as e:
            print(f"Failed to get server info from {self.server_name}: {e}")
            return {}
    
    def init_keygen(self, threshold: int, participants: List[str]) -> Dict:
        """初始化密钥生成"""
        data = {
            "threshold": threshold,
            "participants": participants
        }
        try:
            response = requests.post(f"{self.base_url}/api/v1/keygen", json=data)
            response.raise_for_status()
            return response.json()
        except Exception as e:
            print(f"Failed to init keygen on {self.server_name}: {e}")
            return {}
    
    def init_reshare(self, session_id: str, new_threshold: int, new_participants: List[str]) -> Dict:
        """初始化密钥重分享"""
        data = {
            "session_id": session_id,
            "new_threshold": new_threshold,
            "new_participants": new_participants
        }
        try:
            response = requests.post(f"{self.base_url}/api/v1/reshare", json=data)
            response.raise_for_status()
            return response.json()
        except Exception as e:
            print(f"Failed to init reshare on {self.server_name}: {e}")
            return {}
    
    def init_sign(self, session_id: str, message: str, signers: List[str]) -> Dict:
        """初始化签名"""
        data = {
            "session_id": session_id,
            "message": message,
            "signers": signers
        }
        try:
            response = requests.post(f"{self.base_url}/api/v1/sign", json=data)
            response.raise_for_status()
            return response.json()
        except Exception as e:
            print(f"Failed to init sign on {self.server_name}: {e}")
            return {}
    
    def get_session_status(self, session_id: str) -> Dict:
        """获取会话状态"""
        try:
            response = requests.get(f"{self.base_url}/api/v1/sessions/{session_id}")
            response.raise_for_status()
            return response.json()
        except Exception as e:
            print(f"Failed to get session status from {self.server_name}: {e}")
            return {}
    
    def list_sessions(self) -> Dict:
        """列出所有会话"""
        try:
            response = requests.get(f"{self.base_url}/api/v1/sessions")
            response.raise_for_status()
            return response.json()
        except Exception as e:
            print(f"Failed to list sessions from {self.server_name}: {e}")
            return {}

def wait_for_session_completion(clients: List[MPCClient], session_id: str, timeout: int = 30) -> bool:
    """等待会话完成"""
    start_time = time.time()
    while time.time() - start_time < timeout:
        for client in clients:
            status = client.get_session_status(session_id)
            if status.get("status") == "completed":
                return True
            elif status.get("status") == "failed":
                print(f"Session {session_id} failed")
                return False
        time.sleep(1)
    
    print(f"Session {session_id} timeout after {timeout} seconds")
    return False

def test_keygen(clients: List[MPCClient]) -> Optional[str]:
    """测试密钥生成"""
    print("\n=== 测试密钥生成 ===")
    
    # 使用企业服务器发起密钥生成
    enterprise_client = clients[1]  # enterprise server
    participants = ["third-party", "enterprise", "mobile-app"]
    threshold = 2
    
    print(f"发起密钥生成: threshold={threshold}, participants={participants}")
    result = enterprise_client.init_keygen(threshold, participants)
    
    if not result:
        print("密钥生成发起失败")
        return None
    
    session_id = result.get("session_id")
    print(f"密钥生成会话ID: {session_id}")
    
    # 等待完成
    print("等待密钥生成完成...")
    if wait_for_session_completion(clients, session_id, 10):
        print("✓ 密钥生成成功完成")
        
        # 显示结果
        for client in clients:
            status = client.get_session_status(session_id)
            if status.get("status") == "completed":
                data = status.get("data", {})
                public_key = data.get("public_key", "")
                print(f"  {client.server_name}: 公钥 = {public_key[:32]}...")
        
        return session_id
    else:
        print("✗ 密钥生成失败")
        return None

def test_reshare(clients: List[MPCClient], original_session_id: str) -> Optional[str]:
    """测试密钥重分享"""
    print("\n=== 测试密钥重分享 ===")
    
    # 使用移动应用服务器发起重分享（排除第三方服务器）
    mobile_client = clients[2]  # mobile-app server
    new_participants = ["enterprise", "mobile-app"]
    new_threshold = 2
    
    print(f"发起密钥重分享: new_threshold={new_threshold}, new_participants={new_participants}")
    result = mobile_client.init_reshare(original_session_id, new_threshold, new_participants)
    
    if not result:
        print("密钥重分享发起失败")
        return None
    
    session_id = result.get("session_id")
    print(f"密钥重分享会话ID: {session_id}")
    
    # 等待完成
    print("等待密钥重分享完成...")
    if wait_for_session_completion(clients[1:], session_id, 10):  # 只检查enterprise和mobile-app
        print("✓ 密钥重分享成功完成")
        return session_id
    else:
        print("✗ 密钥重分享失败")
        return None

def test_sign(clients: List[MPCClient], session_id: str):
    """测试签名"""
    print("\n=== 测试签名 ===")
    
    # 使用企业服务器发起签名
    enterprise_client = clients[1]  # enterprise server
    message = "Hello, MPC World!"
    signers = ["enterprise", "mobile-app"]
    
    print(f"发起签名: message='{message}', signers={signers}")
    result = enterprise_client.init_sign(session_id, message, signers)
    
    if not result:
        print("签名发起失败")
        return
    
    sign_session_id = result.get("session_id")
    print(f"签名会话ID: {sign_session_id}")
    
    # 等待完成
    print("等待签名完成...")
    if wait_for_session_completion(clients[1:], sign_session_id, 10):  # 只检查enterprise和mobile-app
        print("✓ 签名成功完成")
        
        # 显示签名结果
        for client in clients[1:]:  # 只检查enterprise和mobile-app
            status = client.get_session_status(sign_session_id)
            if status.get("status") == "completed":
                data = status.get("data", {})
                signature = data.get("signature", "")
                print(f"  {client.server_name}: 签名 = {signature}")
    else:
        print("✗ 签名失败")

def main():
    # 创建客户端
    clients = [
        MPCClient("http://localhost:8081", "third-party"),
        MPCClient("http://localhost:8082", "enterprise"),
        MPCClient("http://localhost:8083", "mobile-app")
    ]
    
    print("MPC Server 测试客户端")
    print("=" * 50)
    
    # 检查服务器状态
    print("检查服务器状态...")
    for client in clients:
        info = client.get_server_info()
        if info:
            print(f"✓ {client.server_name}: {info.get('name', 'Unknown')} - 功能: {info.get('capabilities', [])}")
        else:
            print(f"✗ {client.server_name}: 无法连接")
            sys.exit(1)
    
    # 测试密钥生成
    keygen_session_id = test_keygen(clients)
    if not keygen_session_id:
        print("密钥生成失败，退出测试")
        sys.exit(1)
    
    # 等待一下
    time.sleep(2)
    
    # 测试密钥重分享
    reshare_session_id = test_reshare(clients, keygen_session_id)
    if not reshare_session_id:
        print("密钥重分享失败，跳过签名测试")
    else:
        # 等待一下
        time.sleep(2)
        
        # 测试签名
        test_sign(clients, reshare_session_id)
    
    print("\n=== 测试完成 ===")
    
    # 显示所有会话
    print("\n所有会话列表:")
    for client in clients:
        sessions = client.list_sessions()
        if sessions:
            print(f"\n{client.server_name} 服务器:")
            for session in sessions.get("sessions", []):
                print(f"  - {session['session_id'][:8]}... ({session['type']}) - {session['status']}")

if __name__ == "__main__":
    main()