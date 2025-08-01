#!/usr/bin/env python3
"""
Ed25519 BIP32 + MPC 服务器演示
演示如何在MPC服务器环境中使用Ed25519进行分层确定性密钥派生和签名
"""

import requests
import json
import time
import hashlib
from typing import Dict, List, Optional

class Ed25519BIP32MPCClient:
    def __init__(self, base_url: str, server_name: str):
        self.base_url = base_url
        self.server_name = server_name
        
    def keygen(self, session_id: str, threshold: int = 2, participants: List[str] = None) -> Dict:
        """执行密钥生成"""
        if participants is None:
            participants = ["enterprise", "third-party"]
            
        response = requests.post(f"{self.base_url}/api/v1/keygen", json={
            "session_id": session_id,
            "threshold": threshold,
            "participants": participants
        })
        return response.json()
    
    def sign(self, session_id: str, message: str, signers: List[str] = None) -> Dict:
        """执行签名"""
        if signers is None:
            signers = ["enterprise", "third-party"]
            
        response = requests.post(f"{self.base_url}/api/v1/sign", json={
            "session_id": session_id,
            "message": message,
            "signers": signers
        })
        return response.json()
    
    def get_session_status(self, session_id: str) -> Dict:
        """获取会话状态"""
        response = requests.get(f"{self.base_url}/api/v1/session/{session_id}/status")
        return response.json()
    
    def wait_for_completion(self, session_id: str, timeout: int = 30) -> bool:
        """等待会话完成"""
        start_time = time.time()
        while time.time() - start_time < timeout:
            try:
                status = self.get_session_status(session_id)
                if status.get("status") == "completed":
                    return True
                elif status.get("status") == "failed":
                    print(f"会话 {session_id} 失败: {status.get('error', 'Unknown error')}")
                    return False
                time.sleep(1)
            except Exception as e:
                print(f"检查会话状态时出错: {e}")
                time.sleep(1)
        return False

class Ed25519BIP32KeyManager:
    """Ed25519 BIP32密钥管理器"""
    
    def __init__(self, mpc_client: Ed25519BIP32MPCClient):
        self.mpc_client = mpc_client
        self.master_session_id = None
        self.derived_keys = {}  # 存储派生密钥的会话ID
        
    def generate_master_key(self) -> str:
        """生成主密钥"""
        session_id = f"master_key_{int(time.time())}"
        print(f"🔑 生成Ed25519主密钥 (会话ID: {session_id})")
        
        result = self.mpc_client.keygen(session_id)
        if "session_id" in result:
            print(f"✅ 主密钥生成请求成功")
            if self.mpc_client.wait_for_completion(session_id):
                self.master_session_id = session_id
                print(f"✅ 主密钥生成完成")
                return session_id
            else:
                print(f"❌ 主密钥生成超时")
                return None
        else:
            print(f"❌ 主密钥生成失败: {result}")
            return None
    
    def derive_key_for_purpose(self, purpose: str, path_indices: List[int]) -> str:
        """为特定用途派生密钥"""
        if not self.master_session_id:
            print("❌ 需要先生成主密钥")
            return None
            
        # 在实际实现中，这里应该调用支持BIP32派生的API
        # 目前我们模拟派生过程
        derived_session_id = f"derived_{purpose}_{int(time.time())}"
        path_str = "m/" + "/".join(map(str, path_indices))
        
        print(f"📈 为{purpose}派生密钥 (路径: {path_str})")
        
        # 模拟派生过程 - 在实际实现中，这应该是一个专门的API调用
        # 这里我们使用原始密钥生成来模拟
        result = self.mpc_client.keygen(derived_session_id)
        if "session_id" in result:
            if self.mpc_client.wait_for_completion(derived_session_id):
                self.derived_keys[purpose] = {
                    "session_id": derived_session_id,
                    "path": path_str,
                    "indices": path_indices
                }
                print(f"✅ {purpose}密钥派生完成")
                return derived_session_id
            else:
                print(f"❌ {purpose}密钥派生超时")
                return None
        else:
            print(f"❌ {purpose}密钥派生失败: {result}")
            return None
    
    def sign_with_derived_key(self, purpose: str, message: str) -> Dict:
        """使用派生密钥进行签名"""
        if purpose not in self.derived_keys:
            print(f"❌ 未找到{purpose}的派生密钥")
            return None
            
        key_info = self.derived_keys[purpose]
        session_id = key_info["session_id"]
        path = key_info["path"]
        
        print(f"✍️  使用{purpose}密钥签名 (路径: {path})")
        print(f"   消息: {message}")
        
        # 计算消息哈希
        message_hash = hashlib.sha256(message.encode()).hexdigest()
        print(f"   消息哈希: {message_hash}")
        
        result = self.mpc_client.sign(session_id, message)
        if "session_id" in result:
            sign_session_id = result["session_id"]
            if self.mpc_client.wait_for_completion(sign_session_id):
                print(f"✅ {purpose}签名完成")
                return {
                    "purpose": purpose,
                    "path": path,
                    "message": message,
                    "session_id": sign_session_id,
                    "status": "completed"
                }
            else:
                print(f"❌ {purpose}签名超时")
                return None
        else:
            print(f"❌ {purpose}签名失败: {result}")
            return None

def demonstrate_ed25519_bip32_mpc():
    """演示Ed25519 BIP32 + MPC完整流程"""
    print("=== Ed25519 BIP32 + MPC 服务器演示 ===")
    print("演示在MPC服务器环境中使用Ed25519进行分层确定性密钥派生和签名")
    print()
    
    # 创建MPC客户端
    enterprise_client = Ed25519BIP32MPCClient("http://localhost:8082", "enterprise")
    
    # 创建密钥管理器
    key_manager = Ed25519BIP32KeyManager(enterprise_client)
    
    # 步骤1: 生成主密钥
    print("🔑 步骤1: 生成Ed25519主密钥")
    master_session = key_manager.generate_master_key()
    if not master_session:
        print("❌ 主密钥生成失败，演示终止")
        return
    print()
    
    # 步骤2: 为不同用途派生密钥
    print("📈 步骤2: 为不同用途派生密钥")
    key_purposes = {
        "用户账户": [0, 0],
        "企业钱包": [1, 0], 
        "冷存储": [2, 0],
        "DeFi交互": [3, 0],
        "多签钱包": [4, 0]
    }
    
    derived_sessions = {}
    for purpose, path_indices in key_purposes.items():
        session_id = key_manager.derive_key_for_purpose(purpose, path_indices)
        if session_id:
            derived_sessions[purpose] = session_id
    print()
    
    # 步骤3: 使用派生密钥进行签名
    print("✍️  步骤3: 使用派生密钥进行签名")
    test_messages = {
        "用户账户": "Transfer 100 tokens to Alice",
        "企业钱包": "Corporate payment to supplier",
        "冷存储": "Emergency fund withdrawal",
        "DeFi交互": "Stake 1000 tokens in DeFi protocol",
        "多签钱包": "Multi-signature transaction approval"
    }
    
    signatures = []
    for purpose, message in test_messages.items():
        if purpose in derived_sessions:
            signature_result = key_manager.sign_with_derived_key(purpose, message)
            if signature_result:
                signatures.append(signature_result)
    print()
    
    # 步骤4: 总结结果
    print("📊 步骤4: 演示结果总结")
    print(f"✅ 主密钥生成: 1个")
    print(f"✅ 派生密钥: {len(derived_sessions)}个")
    print(f"✅ 成功签名: {len(signatures)}个")
    print()
    
    print("📋 派生密钥详情:")
    for purpose, key_info in key_manager.derived_keys.items():
        print(f"  • {purpose}: {key_info['path']} (会话ID: {key_info['session_id'][:8]}...)")
    print()
    
    print("📋 签名详情:")
    for sig in signatures:
        print(f"  • {sig['purpose']}: {sig['path']} - {sig['message'][:30]}...")
    print()
    
    # 步骤5: 密钥管理最佳实践
    print("💡 步骤5: Ed25519 BIP32 + MPC 最佳实践")
    print("  🔐 安全性:")
    print("    • 主密钥通过MPC分布式生成，无单点故障")
    print("    • 派生密钥继承MPC的安全属性")
    print("    • 支持确定性密钥派生，便于备份和恢复")
    print()
    print("  🏗️  架构设计:")
    print("    • 为不同用途使用不同的派生路径")
    print("    • 实现分层密钥管理，提高安全性")
    print("    • 支持热钱包和冷钱包的分离")
    print()
    print("  ⚡ 性能优化:")
    print("    • 预先派生常用路径的密钥")
    print("    • 缓存派生密钥信息")
    print("    • 批量处理签名请求")
    print()
    print("  🔄 运维管理:")
    print("    • 定期验证派生密钥的一致性")
    print("    • 监控MPC节点的健康状态")
    print("    • 实现密钥轮换机制")
    print()
    
    print("🎉 Ed25519 BIP32 + MPC 服务器演示完成！")

def check_server_status():
    """检查MPC服务器状态"""
    servers = [
        ("enterprise", "http://localhost:8082"),
        ("third-party", "http://localhost:8081")
    ]
    
    print("🔍 检查MPC服务器状态...")
    all_running = True
    
    for name, url in servers:
        try:
            response = requests.get(f"{url}/health", timeout=5)
            if response.status_code == 200:
                print(f"✅ {name}服务器运行正常")
            else:
                print(f"❌ {name}服务器响应异常: {response.status_code}")
                all_running = False
        except Exception as e:
            print(f"❌ {name}服务器连接失败: {e}")
            all_running = False
    
    return all_running

def main():
    """主函数"""
    print("Ed25519 BIP32 + MPC 服务器演示启动...")
    print()
    
    # 检查服务器状态
    if not check_server_status():
        print("❌ 部分MPC服务器未运行，请先启动服务器")
        print("提示: 运行 'cd mpc-server && ./scripts/start_all.sh' 启动所有服务器")
        return
    
    print()
    
    # 运行演示
    try:
        demonstrate_ed25519_bip32_mpc()
    except KeyboardInterrupt:
        print("\n⏹️  演示被用户中断")
    except Exception as e:
        print(f"\n❌ 演示过程中出现错误: {e}")

if __name__ == "__main__":
    main()