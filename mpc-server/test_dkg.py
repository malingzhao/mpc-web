#!/usr/bin/env python3
import requests
import json
import time

def test_dkg():
    # 服务器配置
    servers = {
        'third-party': 'http://localhost:8081',
        'enterprise': 'http://localhost:8082', 
        'mobile-app': 'http://localhost:8083'
    }
    
    print("=== 测试分布式DKG实现 ===")
    
    # 检查服务器状态
    print("\n1. 检查服务器状态...")
    for name, url in servers.items():
        try:
            response = requests.get(f"{url}/api/v1/info", timeout=5)
            if response.status_code == 200:
                info = response.json()
                print(f"✓ {name}: {info.get('name', 'Unknown')} - 功能: {info.get('capabilities', [])}")
            else:
                print(f"✗ {name}: HTTP {response.status_code}")
        except Exception as e:
            print(f"✗ {name}: 无法连接 - {e}")
    
    # 启动keygen
    print("\n2. 启动DKG keygen...")
    keygen_data = {
        "participants": ["third-party", "enterprise", "mobile-app"],
        "threshold": 2,
        "curve": "secp256k1"
    }
    
    try:
        # 从third-party服务器启动keygen
        response = requests.post(
            f"{servers['third-party']}/api/v1/keygen",
            json=keygen_data,
            timeout=10
        )
        
        if response.status_code == 200:
            result = response.json()
            session_id = result.get('session_id')
            print(f"✓ Keygen启动成功，会话ID: {session_id}")
            
            # 等待一段时间让DKG完成
            print("\n3. 等待DKG完成...")
            for i in range(10):
                time.sleep(2)
                try:
                    status_response = requests.get(
                        f"{servers['third-party']}/api/v1/sessions/{session_id}",
                        timeout=5
                    )
                    if status_response.status_code == 200:
                        status = status_response.json()
                        print(f"   状态: {status.get('status', 'unknown')}")
                        if status.get('status') == 'completed':
                            print("✓ DKG完成！")
                            print(f"   会话详情: {json.dumps(status, indent=2)}")
                            return True
                        elif status.get('status') == 'failed':
                            print("✗ DKG失败")
                            print(f"   错误详情: {json.dumps(status, indent=2)}")
                            return False
                except Exception as e:
                    print(f"   检查状态时出错: {e}")
            
            print("⚠ DKG超时")
            return False
            
        else:
            print(f"✗ Keygen启动失败: HTTP {response.status_code}")
            print(f"   响应: {response.text}")
            return False
            
    except Exception as e:
        print(f"✗ Keygen请求失败: {e}")
        return False

if __name__ == "__main__":
    test_dkg()