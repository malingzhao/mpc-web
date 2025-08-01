package main

import (
	"crypto/elliptic"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"

	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/okx/threshold-lib/crypto/curves"
	"github.com/okx/threshold-lib/tss/key/bip32"
)

func main() {
	fmt.Println("=== Ed25519 阈值签名 + BIP32 密钥派生综合演示 ===")

	// 演示完整的Ed25519阈值签名和BIP32派生流程
	err := demonstrateEd25519ThresholdWithBIP32()
	if err != nil {
		log.Fatalf("演示失败: %v", err)
	}

	fmt.Println("\n=== Ed25519 阈值签名 + BIP32 密钥派生演示完成 ===")
}

func demonstrateEd25519ThresholdWithBIP32() error {
	fmt.Println("\n1. 执行Ed25519 DKG密钥生成...")

	// 设置参数：3方中2方签名
	threshold := 2
	total := 3
	curve := edwards.Edwards()

	// 执行DKG
	shares, publicKey, err := performEd25519DKG(threshold, total, curve)
	if err != nil {
		return fmt.Errorf("DKG失败: %v", err)
	}

	fmt.Printf("DKG成功完成\n")
	fmt.Printf("  阈值: %d/%d\n", threshold, total)
	fmt.Printf("  主公钥X: %s\n", hex.EncodeToString(publicKey.X.Bytes())[:20]+"...")
	fmt.Printf("  主公钥Y: %s\n", hex.EncodeToString(publicKey.Y.Bytes())[:20]+"...")

	fmt.Println("\n2. 为每个参与方创建BIP32主密钥...")

	// 为每个参与方创建BIP32主密钥
	masterChaincode := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	var masterKeys []*bip32.Ed25519TssKey

	for i := 0; i < total; i++ {
		masterKey, err := bip32.NewEd25519TssKey(shares[i], publicKey, masterChaincode)
		if err != nil {
			return fmt.Errorf("创建参与方%d的主密钥失败: %v", i+1, err)
		}
		masterKeys = append(masterKeys, masterKey)

		fmt.Printf("  参与方%d主密钥创建成功\n", i+1)
		fmt.Printf("    私钥份额: %s\n", masterKey.ShareI().String()[:20]+"...")
	}

	fmt.Println("\n3. 派生子密钥 m/44'/60'/0'/0/0...")

	// 派生路径（模拟以太坊地址派生路径，但使用非硬化派生）
	derivationPath := []uint32{44, 60, 0, 0, 0}
	var childKeys []*bip32.Ed25519TssKey

	for i, masterKey := range masterKeys {
		childKey, err := masterKey.DeriveChildKeys(derivationPath)
		if err != nil {
			return fmt.Errorf("参与方%d派生子密钥失败: %v", i+1, err)
		}
		childKeys = append(childKeys, childKey)

		fmt.Printf("  参与方%d子密钥派生成功\n", i+1)
		fmt.Printf("    子私钥份额: %s\n", childKey.ShareI().String()[:20]+"...")
		fmt.Printf("    累积偏移量: %s\n", childKey.PrivateKeyOffset().String()[:20]+"...")
	}

	fmt.Println("\n4. 验证所有参与方的子公钥一致性...")

	// 验证所有参与方派生的子公钥是否一致
	baseChildPublicKey := childKeys[0].PublicKey()
	for i := 1; i < len(childKeys); i++ {
		if !baseChildPublicKey.Equals(childKeys[i].PublicKey()) {
			return fmt.Errorf("参与方%d的子公钥与其他参与方不一致", i+1)
		}
	}

	fmt.Printf("✓ 所有参与方的子公钥一致\n")
	fmt.Printf("  子公钥X: %s\n", hex.EncodeToString(baseChildPublicKey.X.Bytes())[:20]+"...")
	fmt.Printf("  子公钥Y: %s\n", hex.EncodeToString(baseChildPublicKey.Y.Bytes())[:20]+"...")

	fmt.Println("\n5. 使用子密钥进行阈值签名...")

	// 模拟阈值签名（使用参与方1和2）
	message := []byte("Hello Ed25519 Threshold + BIP32!")
	participants := []int{0, 1} // 参与方1和2

	fmt.Printf("  消息: %s\n", string(message))
	fmt.Printf("  参与签名的方: %v\n", []int{participants[0] + 1, participants[1] + 1})

	// 这里我们模拟签名过程，实际实现需要完整的阈值签名协议
	fmt.Printf("✓ 阈值签名模拟成功（实际实现需要完整的签名协议）\n")

	fmt.Println("\n6. 演示多层级密钥派生...")

	// 演示不同的派生路径
	derivationPaths := [][]uint32{
		{44, 60, 0, 0, 0}, // 第一个以太坊地址
		{44, 60, 0, 0, 1}, // 第二个以太坊地址
		{44, 60, 1, 0, 0}, // 第二个账户的第一个地址
		{44, 0, 0, 0, 0},  // 比特币地址（模拟）
	}

	for pathIdx, path := range derivationPaths {
		fmt.Printf("\n  派生路径 m")
		for _, idx := range path {
			fmt.Printf("/%d", idx)
		}
		fmt.Printf(":\n")

		// 为第一个参与方派生此路径
		derivedKey, err := masterKeys[0].DeriveChildKeys(path)
		if err != nil {
			return fmt.Errorf("派生路径%d失败: %v", pathIdx, err)
		}

		fmt.Printf("    公钥X: %s\n", hex.EncodeToString(derivedKey.PublicKey().X.Bytes())[:20]+"...")
		fmt.Printf("    公钥Y: %s\n", hex.EncodeToString(derivedKey.PublicKey().Y.Bytes())[:20]+"...")
		fmt.Printf("    偏移量: %s\n", derivedKey.PrivateKeyOffset().String()[:20]+"...")

		// 转换为Ed25519公钥对象
		ed25519PubKey := derivedKey.ToEd25519PublicKey()
		fmt.Printf("    Ed25519公钥: %s\n", hex.EncodeToString(ed25519PubKey.SerializeCompressed())[:20]+"...")
	}

	fmt.Println("\n7. 性能测试：批量派生...")

	// 测试批量派生性能
	testPaths := [][]uint32{
		{0}, {1}, {2}, {3}, {4},
		{0, 0}, {0, 1}, {0, 2},
		{1, 0}, {1, 1}, {1, 2},
		{0, 0, 0}, {0, 0, 1}, {0, 1, 0},
	}

	fmt.Printf("  批量派生%d个不同路径...\n", len(testPaths))

	for i, path := range testPaths {
		_, err := masterKeys[0].DeriveChildKeys(path)
		if err != nil {
			return fmt.Errorf("批量派生第%d个路径失败: %v", i+1, err)
		}
	}

	fmt.Printf("✓ 批量派生%d个路径全部成功\n", len(testPaths))

	return nil
}

// 简化的DKG实现（实际应用中需要使用完整的DKG协议）
func performEd25519DKG(threshold, total int, curve elliptic.Curve) ([]*big.Int, *curves.ECPoint, error) {
	// 这是一个简化的DKG模拟，实际应用中需要使用完整的分布式密钥生成协议

	// 生成主私钥（在真实DKG中，这个值不会被任何单方知道）
	masterSecret := new(big.Int)
	masterSecret.SetString("987654321098765432109876543210987654321098765432109876543210", 10)
	masterSecret = new(big.Int).Mod(masterSecret, curve.Params().N)

	// 计算主公钥
	masterPublicKey := curves.ScalarToPoint(curve, masterSecret)

	// 生成份额（简化版本，实际DKG会使用Shamir秘密分享）
	var shares []*big.Int
	for i := 0; i < total; i++ {
		// 简化的份额生成（实际应该使用Shamir秘密分享）
		share := new(big.Int).Add(masterSecret, big.NewInt(int64(i+1)))
		share = new(big.Int).Mod(share, curve.Params().N)
		shares = append(shares, share)
	}

	return shares, masterPublicKey, nil
}
