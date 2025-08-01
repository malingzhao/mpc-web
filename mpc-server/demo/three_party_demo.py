#!/usr/bin/env python3
"""
三方MPC服务器演示
演示A、B、C三台服务器通过协调节点进行密钥生成和签名的完整流程
"""

import asyncio
import websockets
import json
import time
import threading
import requests
from typing import Dict, List, Optional
import logging

# 配置日志
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

class MPCClient:
    """MPC客户端类"""
    
    def __init__(self, client_id: str, server_url: str, api_url: str):
        self.client_id = client_id
        self.server_url = server_url
        self.api_url = api_url
        self.websocket = None
        self.connected = False
        self.session_id = None
        self.public_key = None
        self.messages = []
        
    async def connect(self):
        """连接到WebSocket服务器"""
        try:
            uri = f"{self.server_url}?client_id={self.client_id}"
            self.websocket = await websockets.connect(uri)
            self.connected = True
            logger.info(f"客户端 {self.client_id} 已连接到 {self.server_url}")
            
            # 启动消息监听
            asyncio.create_task(self.listen_messages())
            
        except Exception as e:
            logger.error(f"客户端 {self.client_id} 连接失败: {e}")
            
    async def listen_messages(self):
        """监听WebSocket消息"""
        try:
            async for message in self.websocket:
                data = json.loads(message)
                self.messages.append(data)
                await self.handle_message(data)
        except Exception as e:
            logger.error(f"客户端 {self.client_id} 消息监听错误: {e}")
            
    async def handle_message(self, data: dict):
        """处理收到的消息"""
        msg_type = data.get('type')
        logger.info(f"客户端 {self.client_id} 收到消息: {msg_type}")
        
        if msg_type == 'keygen_complete':
            self.public_key = data.get('public_key')
            logger.info(f"客户端 {self.client_id} 密钥生成完成，公钥: {self.public_key[:40]}...")
            
        elif msg_type == 'sign_complete':
            signature = data.get('signature')
            logger.info(f"客户端 {self.client_id} 签名完成: {signature}")
            
        elif msg_type == 'session_failed':
            error = data.get('error', 'Unknown error')
            logger.error(f"客户端 {self.client_id} 会话失败: {error}")
            
    async def send_message(self, message: dict):
        """发送WebSocket消息"""
        if self.websocket and self.connected:
            await self.websocket.send(json.dumps(message))
            
    def request_keygen(self, participants: List[str], threshold: int) -> dict:
        """请求密钥生成"""
        try:
            url = f"{self.api_url}/keygen"
            payload = {
                "participants": participants,
                "threshold": threshold
            }
            response = requests.post(url, json=payload, timeout=10)
            result = response.json()
            
            if response.status_code == 200:
                self.session_id = result.get('session_id')
                logger.info(f"客户端 {self.client_id} 密钥生成请求成功，会话ID: {self.session_id}")
            else:
                logger.error(f"客户端 {self.client_id} 密钥生成请求失败: {result}")
                
            return result
        except Exception as e:
            logger.error(f"客户端 {self.client_id} 密钥生成请求异常: {e}")
            return {"error": str(e)}
            
    def request_sign(self, message: str, participants: List[str]) -> dict:
        """请求签名"""
        try:
            url = f"{self.api_url}/sign"
            payload = {
                "message": message,
                "participants": participants,
                "session_id": self.session_id
            }
            response = requests.post(url, json=payload, timeout=10)
            result = response.json()
            
            if response.status_code == 200:
                logger.info(f"客户端 {self.client_id} 签名请求成功")
            else:
                logger.error(f"客户端 {self.client_id} 签名请求失败: {result}")
                
            return result
        except Exception as e:
            logger.error(f"客户端 {self.client_id} 签名请求异常: {e}")
            return {"error": str(e)}
            
    async def disconnect(self):
        """断开连接"""
        if self.websocket:
            await self.websocket.close()
            self.connected = False
            logger.info(f"客户端 {self.client_id} 已断开连接")

class ThreePartyDemo:
    """三方MPC演示类"""
    
    def __init__(self):
        # 服务器配置
        self.servers = {
            "enterprise": {
                "ws_url": "ws://localhost:8082/ws",
                "api_url": "http://localhost:8082/api/v1"
            },
            "mobile": {
                "ws_url": "ws://localhost:8083/ws", 
                "api_url": "http://localhost:8083/api/v1"
            },
            "third_party": {
                "ws_url": "ws://localhost:8084/ws",
                "api_url": "http://localhost:8084/api/v1"
            }
        }
        
        # 创建客户端
        self.clients = {}
        for server_name, config in self.servers.items():
            self.clients[server_name] = MPCClient(
                client_id=server_name,
                server_url=config["ws_url"],
                api_url=config["api_url"]
            )
            
    async def connect_all_clients(self):
        """连接所有客户端"""
        logger.info("正在连接所有客户端...")
        tasks = []
        for client in self.clients.values():
            tasks.append(client.connect())
        await asyncio.gather(*tasks)
        
        # 等待连接稳定
        await asyncio.sleep(2)
        logger.info("所有客户端连接完成")
        
    async def run_keygen_demo(self):
        """运行密钥生成演示"""
        logger.info("=" * 50)
        logger.info("开始三方密钥生成演示")
        logger.info("=" * 50)
        
        participants = ["enterprise", "mobile", "third_party"]
        threshold = 2  # 2-of-3 threshold
        
        # 由enterprise节点发起密钥生成
        coordinator = self.clients["enterprise"]
        result = coordinator.request_keygen(participants, threshold)
        
        if "error" in result:
            logger.error(f"密钥生成失败: {result['error']}")
            return False
            
        # 等待密钥生成完成
        logger.info("等待密钥生成完成...")
        await self.wait_for_keygen_completion()
        
        return True
        
    async def wait_for_keygen_completion(self, timeout: int = 30):
        """等待密钥生成完成"""
        start_time = time.time()
        while time.time() - start_time < timeout:
            # 检查所有客户端是否都完成了密钥生成
            completed_count = 0
            for client in self.clients.values():
                if client.public_key:
                    completed_count += 1
                    
            if completed_count == len(self.clients):
                logger.info("所有客户端密钥生成完成")
                return True
                
            await asyncio.sleep(1)
            
        logger.error("密钥生成超时")
        return False
        
    async def run_sign_demo(self):
        """运行签名演示"""
        logger.info("=" * 50)
        logger.info("开始三方签名演示")
        logger.info("=" * 50)
        
        message = "Hello, MPC World!"
        participants = ["enterprise", "mobile", "third_party"]
        
        # 由enterprise节点发起签名
        coordinator = self.clients["enterprise"]
        result = coordinator.request_sign(message, participants)
        
        if "error" in result:
            logger.error(f"签名失败: {result['error']}")
            return False
            
        # 等待签名完成
        logger.info(f"等待对消息 '{message}' 的签名完成...")
        await self.wait_for_sign_completion()
        
        return True
        
    async def wait_for_sign_completion(self, timeout: int = 30):
        """等待签名完成"""
        start_time = time.time()
        while time.time() - start_time < timeout:
            # 检查是否有客户端收到签名完成消息
            for client in self.clients.values():
                for msg in client.messages:
                    if msg.get('type') == 'sign_complete':
                        logger.info("签名完成")
                        return True
                        
            await asyncio.sleep(1)
            
        logger.error("签名超时")
        return False
        
    async def disconnect_all_clients(self):
        """断开所有客户端连接"""
        logger.info("正在断开所有客户端连接...")
        tasks = []
        for client in self.clients.values():
            tasks.append(client.disconnect())
        await asyncio.gather(*tasks)
        logger.info("所有客户端已断开连接")
        
    async def run_demo(self):
        """运行完整演示"""
        try:
            # 连接所有客户端
            await self.connect_all_clients()
            
            # 运行密钥生成演示
            keygen_success = await self.run_keygen_demo()
            if not keygen_success:
                logger.error("密钥生成演示失败")
                return
                
            # 等待一段时间
            await asyncio.sleep(3)
            
            # 运行签名演示
            sign_success = await self.run_sign_demo()
            if not sign_success:
                logger.error("签名演示失败")
                return
                
            logger.info("=" * 50)
            logger.info("三方MPC演示完成！")
            logger.info("=" * 50)
            
        except Exception as e:
            logger.error(f"演示过程中发生错误: {e}")
        finally:
            # 断开所有连接
            await self.disconnect_all_clients()

def check_servers():
    """检查服务器是否运行"""
    servers = [
        "http://localhost:8082/api/v1/health",
        "http://localhost:8083/api/v1/health", 
        "http://localhost:8084/api/v1/health"
    ]
    
    for i, server in enumerate(servers):
        try:
            response = requests.get(server, timeout=5)
            if response.status_code == 200:
                logger.info(f"服务器 {['Enterprise', 'Mobile', 'Third-party'][i]} 运行正常")
            else:
                logger.warning(f"服务器 {['Enterprise', 'Mobile', 'Third-party'][i]} 响应异常")
        except Exception as e:
            logger.error(f"服务器 {['Enterprise', 'Mobile', 'Third-party'][i]} 无法连接: {e}")

async def main():
    """主函数"""
    logger.info("三方MPC服务器演示程序")
    logger.info("=" * 50)
    
    # 检查服务器状态
    logger.info("检查服务器状态...")
    check_servers()
    
    # 运行演示
    demo = ThreePartyDemo()
    await demo.run_demo()

if __name__ == "__main__":
    asyncio.run(main())