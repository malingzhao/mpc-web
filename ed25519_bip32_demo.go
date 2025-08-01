package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/big"

	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/okx/threshold-lib/crypto/curves"
	"github.com/okx/threshold-lib/tss/key/bip32"
)

func main() {
	fmt.Println("=== Ed25519 BIP32 密钥派生演示 ===")
	
	// 演示Ed25519 BIP32密钥派生
	err := demonstrateEd25519BIP32()
	if err != nil {
		log.Fatalf("Ed25519 BIP32演示失败: %v", err)
	}
	
	fmt.Println("\n=== Ed25519 BIP32密钥派生演示完成 ===")
}

func demonstrateEd25519BIP32() error {
	fmt.Println("\n1. 创建Ed25519主密钥...")
	
	// 创建Ed25519曲线
	curve := edwards.Edwards()
	
	// 生成主私钥（在实际应用中，这应该来自安全的随机数生成器或DKG）
	masterPrivateKey := big.NewInt(0)
	masterPrivateKey.SetString("123456789012345678901234567890123456789012345678901234567890", 10)
	
	// 计算对应的公钥点
	masterPublicKey := curves.ScalarToPoint(curve, masterPrivateKey)
	
	// 主链码（通常来自种子的HMAC-SHA512）
	masterChaincode := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	
	// 创建主TSS密钥
	masterTssKey, err := bip32.NewEd25519TssKey(masterPrivateKey, masterPublicKey, masterChaincode)
	if err != nil {
		return fmt.Errorf("创建主密钥失败: %v", err)
	}
	
	fmt.Printf("主密钥创建成功\n")
	fmt.Printf("  私钥份额: %s\n", masterTssKey.ShareI().String()[:20]+"...")
	fmt.Printf("  公钥X: %s\n", hex.EncodeToString(masterTssKey.PublicKey().X.Bytes())[:20]+"...")
	fmt.Printf("  公钥Y: %s\n", hex.EncodeToString(masterTssKey.PublicKey().Y.Bytes())[:20]+"...")
	fmt.Printf("  链码: %s\n", hex.EncodeToString(masterTssKey.Chaincode())[:20]+"...")
	
	fmt.Println("\n2. 派生子密钥 m/0...")
	
	// 派生第一个子密钥 m/0
	child0, err := masterTssKey.NewChildKey(0)
	if err != nil {
		return fmt.Errorf("派生子密钥m/0失败: %v", err)
	}
	
	fmt.Printf("子密钥m/0创建成功\n")
	fmt.Printf("  私钥份额: %s\n", child0.ShareI().String()[:20]+"...")
	fmt.Printf("  公钥X: %s\n", hex.EncodeToString(child0.PublicKey().X.Bytes())[:20]+"...")
	fmt.Printf("  公钥Y: %s\n", hex.EncodeToString(child0.PublicKey().Y.Bytes())[:20]+"...")
	fmt.Printf("  偏移量: %s\n", child0.PrivateKeyOffset().String()[:20]+"...")
	
	fmt.Println("\n3. 继续派生子密钥 m/0/1...")
	
	// 派生第二层子密钥 m/0/1
	child01, err := child0.NewChildKey(1)
	if err != nil {
		return fmt.Errorf("派生子密钥m/0/1失败: %v", err)
	}
	
	fmt.Printf("子密钥m/0/1创建成功\n")
	fmt.Printf("  私钥份额: %s\n", child01.ShareI().String()[:20]+"...")
	fmt.Printf("  公钥X: %s\n", hex.EncodeToString(child01.PublicKey().X.Bytes())[:20]+"...")
	fmt.Printf("  公钥Y: %s\n", hex.EncodeToString(child01.PublicKey().Y.Bytes())[:20]+"...")
	fmt.Printf("  累积偏移量: %s\n", child01.PrivateKeyOffset().String()[:20]+"...")
	
	fmt.Println("\n4. 批量派生密钥 m/0/1/2/3...")
	
	// 使用批量派生功能
	path := []uint32{0, 1, 2, 3}
	batchDerived, err := masterTssKey.DeriveChildKeys(path)
	if err != nil {
		return fmt.Errorf("批量派生失败: %v", err)
	}
	
	fmt.Printf("批量派生m/0/1/2/3成功\n")
	fmt.Printf("  私钥份额: %s\n", batchDerived.ShareI().String()[:20]+"...")
	fmt.Printf("  公钥X: %s\n", hex.EncodeToString(batchDerived.PublicKey().X.Bytes())[:20]+"...")
	fmt.Printf("  公钥Y: %s\n", hex.EncodeToString(batchDerived.PublicKey().Y.Bytes())[:20]+"...")
	fmt.Printf("  累积偏移量: %s\n", batchDerived.PrivateKeyOffset().String()[:20]+"...")
	fmt.Printf("  派生路径信息: %s\n", batchDerived.GetDerivationPath())
	
	fmt.Println("\n5. 验证逐步派生与批量派生的一致性...")
	
	// 逐步派生相同路径
	step1, err := masterTssKey.NewChildKey(0)
	if err != nil {
		return err
	}
	step2, err := step1.NewChildKey(1)
	if err != nil {
		return err
	}
	step3, err := step2.NewChildKey(2)
	if err != nil {
		return err
	}
	step4, err := step3.NewChildKey(3)
	if err != nil {
		return err
	}
	
	// 比较结果
	if step4.ShareI().Cmp(batchDerived.ShareI()) == 0 &&
		step4.PublicKey().X.Cmp(batchDerived.PublicKey().X) == 0 &&
		step4.PublicKey().Y.Cmp(batchDerived.PublicKey().Y) == 0 {
		fmt.Println("✓ 逐步派生与批量派生结果一致")
	} else {
		fmt.Println("✗ 逐步派生与批量派生结果不一致")
	}
	
	fmt.Println("\n6. 转换为Ed25519公钥对象...")
	
	// 转换为标准Ed25519公钥
	ed25519PubKey := batchDerived.ToEd25519PublicKey()
	fmt.Printf("Ed25519公钥转换成功\n")
	fmt.Printf("  公钥X坐标: %s\n", hex.EncodeToString(ed25519PubKey.GetX().Bytes())[:20]+"...")
	fmt.Printf("  公钥Y坐标: %s\n", hex.EncodeToString(ed25519PubKey.GetY().Bytes())[:20]+"...")
	
	fmt.Println("\n7. 测试硬化派生限制...")
	
	// 尝试硬化派生（应该失败）
	hardenedIndex := uint32(0x80000000) // 2^31
	_, err = masterTssKey.NewChildKey(hardenedIndex)
	if err != nil {
		fmt.Printf("✓ 硬化派生正确被拒绝: %v\n", err)
	} else {
		fmt.Println("✗ 硬化派生应该被拒绝但没有")
	}
	
	fmt.Println("\n8. 演示多个不同的派生路径...")
	
	// 派生多个不同路径
	paths := [][]uint32{
		{0},
		{1},
		{0, 0},
		{0, 1},
		{1, 0},
		{0, 1, 2},
		{1, 2, 3},
	}
	
	for _, path := range paths {
		derived, err := masterTssKey.DeriveChildKeys(path)
		if err != nil {
			return fmt.Errorf("派生路径%v失败: %v", path, err)
		}
		
		pathStr := "m"
		for _, idx := range path {
			pathStr += fmt.Sprintf("/%d", idx)
		}
		
		fmt.Printf("  路径%s: 公钥X=%s...\n", 
			pathStr, 
			hex.EncodeToString(derived.PublicKey().X.Bytes())[:16])
	}
	
	return nil
}