package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"

	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/okx/threshold-lib/tss"
	"github.com/okx/threshold-lib/tss/ed25519/sign"
	"github.com/okx/threshold-lib/tss/key/bip32"
	"github.com/okx/threshold-lib/tss/key/dkg"
)

var (
	curve = edwards.Edwards()
)

func main() {
	fmt.Println("=== Ed25519 BIP32 + MPC 完整演示 ===")
	fmt.Println("演示在MPC环境中使用Ed25519进行分层确定性密钥派生和签名")
	fmt.Println()

	// 步骤1: MPC分布式密钥生成
	fmt.Println("🔑 步骤1: Ed25519 MPC分布式密钥生成")
	p1Data, p2Data, p3Data := performEd25519MPCDKG()

	// 步骤2: 创建主TSS密钥用于BIP32派生
	fmt.Println("🌳 步骤2: 创建Ed25519 BIP32主密钥")
	masterTssKey1, masterTssKey2, masterTssKey3 := createMasterTssKeys(p1Data, p2Data, p3Data)

	// 步骤3: 演示BIP32密钥派生
	fmt.Println("📈 步骤3: Ed25519 BIP32密钥派生演示")
	demonstrateBIP32Derivation(masterTssKey1, masterTssKey2, masterTssKey3)

	// 步骤4: 使用派生密钥进行MPC签名
	fmt.Println("✍️  步骤4: 使用派生密钥进行MPC签名")
	demonstrateMPCSigningWithDerivedKeys(p1Data, p2Data, p3Data, masterTssKey1, masterTssKey2)

	// 步骤5: 多路径密钥管理演示
	fmt.Println("🗂️  步骤5: 多路径密钥管理演示")
	demonstrateMultiPathKeyManagement(masterTssKey1, masterTssKey2, p1Data, p2Data)

	fmt.Println("\n🎉 Ed25519 BIP32 + MPC 完整演示完成！")
}

// performEd25519MPCDKG 执行Ed25519 MPC分布式密钥生成
func performEd25519MPCDKG() (*tss.KeyStep3Data, *tss.KeyStep3Data, *tss.KeyStep3Data) {
	fmt.Println("执行3方Ed25519 MPC DKG密钥生成...")

	// 初始化3个参与者，使用Edwards曲线
	setUp1 := dkg.NewSetUp(1, 3, curve)
	setUp2 := dkg.NewSetUp(2, 3, curve)
	setUp3 := dkg.NewSetUp(3, 3, curve)

	// DKG第一轮
	msgs1_1, _ := setUp1.DKGStep1()
	msgs2_1, _ := setUp2.DKGStep1()
	msgs3_1, _ := setUp3.DKGStep1()

	// 构造第二轮输入消息
	msgs1_2_in := []*tss.Message{msgs2_1[1], msgs3_1[1]}
	msgs2_2_in := []*tss.Message{msgs1_1[2], msgs3_1[2]}
	msgs3_2_in := []*tss.Message{msgs1_1[3], msgs2_1[3]}

	// DKG第二轮
	msgs1_2, _ := setUp1.DKGStep2(msgs1_2_in)
	msgs2_2, _ := setUp2.DKGStep2(msgs2_2_in)
	msgs3_2, _ := setUp3.DKGStep2(msgs3_2_in)

	// 构造第三轮输入消息
	msgs1_3_in := []*tss.Message{msgs2_2[1], msgs3_2[1]}
	msgs2_3_in := []*tss.Message{msgs1_2[2], msgs3_2[2]}
	msgs3_3_in := []*tss.Message{msgs1_2[3], msgs2_2[3]}

	// DKG第三轮 - 完成密钥生成
	p1SaveData, _ := setUp1.DKGStep3(msgs1_3_in)
	p2SaveData, _ := setUp2.DKGStep3(msgs2_3_in)
	p3SaveData, _ := setUp3.DKGStep3(msgs3_3_in)

	fmt.Printf("✅ MPC DKG完成\n")
	fmt.Printf("  参与者1: ID=%d, 密钥份额=%s...\n", 
		p1SaveData.Id, p1SaveData.ShareI.String()[:20])
	fmt.Printf("  参与者2: ID=%d, 密钥份额=%s...\n", 
		p2SaveData.Id, p2SaveData.ShareI.String()[:20])
	fmt.Printf("  参与者3: ID=%d, 密钥份额=%s...\n", 
		p3SaveData.Id, p3SaveData.ShareI.String()[:20])
	fmt.Printf("  共同公钥: (%s..., %s...)\n", 
		p1SaveData.PublicKey.X.String()[:20], 
		p1SaveData.PublicKey.Y.String()[:20])
	fmt.Println()

	return p1SaveData, p2SaveData, p3SaveData
}

// createMasterTssKeys 创建主TSS密钥用于BIP32派生
func createMasterTssKeys(p1Data, p2Data, p3Data *tss.KeyStep3Data) (*bip32.Ed25519TssKey, *bip32.Ed25519TssKey, *bip32.Ed25519TssKey) {
	// 生成主链码（在实际应用中，这应该来自安全的种子）
	masterChaincode := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	
	// 为每个参与者创建TSS密钥
	masterTssKey1, err := bip32.NewEd25519TssKey(p1Data.ShareI, p1Data.PublicKey, masterChaincode)
	if err != nil {
		log.Fatalf("创建参与者1的主TSS密钥失败: %v", err)
	}

	masterTssKey2, err := bip32.NewEd25519TssKey(p2Data.ShareI, p2Data.PublicKey, masterChaincode)
	if err != nil {
		log.Fatalf("创建参与者2的主TSS密钥失败: %v", err)
	}

	masterTssKey3, err := bip32.NewEd25519TssKey(p3Data.ShareI, p3Data.PublicKey, masterChaincode)
	if err != nil {
		log.Fatalf("创建参与者3的主TSS密钥失败: %v", err)
	}

	fmt.Printf("✅ 主TSS密钥创建成功\n")
	fmt.Printf("  主链码: %s\n", masterChaincode[:20]+"...")
	fmt.Printf("  主公钥: (%s..., %s...)\n", 
		masterTssKey1.PublicKey().X.String()[:20], 
		masterTssKey1.PublicKey().Y.String()[:20])
	fmt.Println()

	return masterTssKey1, masterTssKey2, masterTssKey3
}

// demonstrateBIP32Derivation 演示BIP32密钥派生
func demonstrateBIP32Derivation(masterTssKey1, masterTssKey2, masterTssKey3 *bip32.Ed25519TssKey) {
	fmt.Println("演示Ed25519 BIP32密钥派生...")

	// 定义派生路径
	derivationPaths := [][]uint32{
		{0},           // m/0
		{0, 1},        // m/0/1
		{0, 1, 2},     // m/0/1/2
		{1, 0},        // m/1/0
		{44, 0, 0},    // m/44/0/0 (类似BIP44路径，但非硬化)
	}

	for _, path := range derivationPaths {
		// 为每个参与者派生相同路径的子密钥
		child1, err := masterTssKey1.DeriveChildKeys(path)
		if err != nil {
			log.Printf("参与者1派生路径%v失败: %v", path, err)
			continue
		}

		child2, err := masterTssKey2.DeriveChildKeys(path)
		if err != nil {
			log.Printf("参与者2派生路径%v失败: %v", path, err)
			continue
		}

		child3, err := masterTssKey3.DeriveChildKeys(path)
		if err != nil {
			log.Printf("参与者3派生路径%v失败: %v", path, err)
			continue
		}

		// 验证派生的公钥一致性
		pubKeyMatch := child1.PublicKey().X.Cmp(child2.PublicKey().X) == 0 && 
			child1.PublicKey().Y.Cmp(child2.PublicKey().Y) == 0 &&
			child2.PublicKey().X.Cmp(child3.PublicKey().X) == 0 && 
			child2.PublicKey().Y.Cmp(child3.PublicKey().Y) == 0

		pathStr := "m"
		for _, idx := range path {
			pathStr += fmt.Sprintf("/%d", idx)
		}

		fmt.Printf("  路径%s: 公钥一致性=%s, 公钥=(%s..., %s...)\n", 
			pathStr,
			map[bool]string{true: "✅", false: "❌"}[pubKeyMatch],
			child1.PublicKey().X.String()[:16],
			child1.PublicKey().Y.String()[:16])
	}
	fmt.Println()
}

// demonstrateMPCSigningWithDerivedKeys 使用派生密钥进行MPC签名
func demonstrateMPCSigningWithDerivedKeys(p1Data, p2Data, p3Data *tss.KeyStep3Data, masterTssKey1, masterTssKey2 *bip32.Ed25519TssKey) {
	fmt.Println("使用派生密钥进行MPC签名...")

	// 派生用于签名的子密钥 m/0/1
	path := []uint32{0, 1}
	child1, err := masterTssKey1.DeriveChildKeys(path)
	if err != nil {
		log.Fatalf("派生子密钥失败: %v", err)
	}

	child2, err := masterTssKey2.DeriveChildKeys(path)
	if err != nil {
		log.Fatalf("派生子密钥失败: %v", err)
	}

	// 计算调整后的密钥份额（原始份额 + 派生偏移量）
	adjustedShare1 := new(big.Int).Add(p1Data.ShareI, child1.PrivateKeyOffset())
	adjustedShare1 = new(big.Int).Mod(adjustedShare1, curve.Params().N)

	adjustedShare2 := new(big.Int).Add(p2Data.ShareI, child2.PrivateKeyOffset())
	adjustedShare2 = new(big.Int).Mod(adjustedShare2, curve.Params().N)

	// 要签名的消息
	message := "Hello from Ed25519 BIP32 + MPC!"
	hash := sha256.New()
	hash.Write([]byte(message))
	messageHash := hash.Sum(nil)
	messageHex := hex.EncodeToString(messageHash)

	fmt.Printf("  签名消息: %s\n", message)
	fmt.Printf("  消息哈希: %s\n", messageHex)
	fmt.Printf("  使用路径: m/0/1\n")

	// 创建Ed25519公钥
	publicKey := edwards.NewPublicKey(child1.PublicKey().X, child1.PublicKey().Y)

	// 初始化签名参与者（使用调整后的密钥份额）
	partList := []int{1, 2}
	p1 := sign.NewEd25519Sign(1, 2, partList, adjustedShare1, publicKey, messageHex)
	p2 := sign.NewEd25519Sign(2, 2, partList, adjustedShare2, publicKey, messageHex)

	// 执行MPC签名协议
	// 签名第一步
	p1Step1, err := p1.SignStep1()
	if err != nil {
		log.Fatalf("P1 Step1失败: %v", err)
	}

	p2Step1, err := p2.SignStep1()
	if err != nil {
		log.Fatalf("P2 Step1失败: %v", err)
	}

	// 签名第二步
	p1Step2, err := p1.SignStep2([]*tss.Message{p2Step1[1]})
	if err != nil {
		log.Fatalf("P1 Step2失败: %v", err)
	}

	p2Step2, err := p2.SignStep2([]*tss.Message{p1Step1[2]})
	if err != nil {
		log.Fatalf("P2 Step2失败: %v", err)
	}

	// 签名第三步 - 完成签名
	si_1, r, err := p1.SignStep3([]*tss.Message{p2Step2[1]})
	if err != nil {
		log.Fatalf("P1 Step3失败: %v", err)
	}

	si_2, _, err := p2.SignStep3([]*tss.Message{p1Step2[2]})
	if err != nil {
		log.Fatalf("P2 Step3失败: %v", err)
	}

	// 合并签名
	s := new(big.Int).Add(si_1, si_2)
	fmt.Printf("  MPC签名完成: r=%s..., s=%s...\n", r.String()[:20], s.String()[:20])

	// 验证签名
	signature := edwards.NewSignature(r, s)
	valid := signature.Verify(messageHash, publicKey)
	fmt.Printf("  签名验证: %s\n", map[bool]string{true: "✅ 有效", false: "❌ 无效"}[valid])
	fmt.Println()
}

// demonstrateMultiPathKeyManagement 演示多路径密钥管理
func demonstrateMultiPathKeyManagement(masterTssKey1, masterTssKey2 *bip32.Ed25519TssKey, p1Data, p2Data *tss.KeyStep3Data) {
	fmt.Println("演示多路径密钥管理...")

	// 模拟不同用途的密钥路径
	keyPurposes := map[string][]uint32{
		"用户账户1":     {0, 0},
		"用户账户2":     {0, 1},
		"企业钱包":      {1, 0},
		"冷存储":       {2, 0},
		"多签钱包":      {3, 0},
		"DeFi交互":    {4, 0},
	}

	fmt.Println("  不同用途的密钥派生:")
	for purpose, path := range keyPurposes {
		// 派生密钥
		child1, err := masterTssKey1.DeriveChildKeys(path)
		if err != nil {
			log.Printf("派生%s密钥失败: %v", purpose, err)
			continue
		}

		child2, err := masterTssKey2.DeriveChildKeys(path)
		if err != nil {
			log.Printf("派生%s密钥失败: %v", purpose, err)
			continue
		}

		// 验证公钥一致性
		pubKeyMatch := child1.PublicKey().X.Cmp(child2.PublicKey().X) == 0 && 
			child1.PublicKey().Y.Cmp(child2.PublicKey().Y) == 0

		pathStr := "m"
		for _, idx := range path {
			pathStr += fmt.Sprintf("/%d", idx)
		}

		fmt.Printf("    %s (%s): %s\n", 
			purpose, 
			pathStr,
			map[bool]string{true: "✅ 密钥一致", false: "❌ 密钥不一致"}[pubKeyMatch])

		// 演示快速签名验证
		if pubKeyMatch {
			testMessage := fmt.Sprintf("Test transaction for %s", purpose)
			success := performQuickSignTest(child1, child2, p1Data, p2Data, testMessage)
			fmt.Printf("      快速签名测试: %s\n", 
				map[bool]string{true: "✅ 成功", false: "❌ 失败"}[success])
		}
	}

	fmt.Println("\n  密钥管理最佳实践:")
	fmt.Println("    • 为不同用途使用不同的派生路径")
	fmt.Println("    • 保持主密钥的安全性")
	fmt.Println("    • 定期验证派生密钥的一致性")
	fmt.Println("    • 使用确定性派生确保可重现性")
	fmt.Println()
}

// performQuickSignTest 执行快速签名测试
func performQuickSignTest(child1, child2 *bip32.Ed25519TssKey, p1Data, p2Data *tss.KeyStep3Data, message string) bool {
	// 计算调整后的密钥份额
	adjustedShare1 := new(big.Int).Add(p1Data.ShareI, child1.PrivateKeyOffset())
	adjustedShare1 = new(big.Int).Mod(adjustedShare1, curve.Params().N)

	adjustedShare2 := new(big.Int).Add(p2Data.ShareI, child2.PrivateKeyOffset())
	adjustedShare2 = new(big.Int).Mod(adjustedShare2, curve.Params().N)

	// 计算消息哈希
	hash := sha256.New()
	hash.Write([]byte(message))
	messageHash := hash.Sum(nil)
	messageHex := hex.EncodeToString(messageHash)

	// 创建Ed25519公钥
	publicKey := edwards.NewPublicKey(child1.PublicKey().X, child1.PublicKey().Y)

	// 初始化签名参与者
	partList := []int{1, 2}
	p1 := sign.NewEd25519Sign(1, 2, partList, adjustedShare1, publicKey, messageHex)
	p2 := sign.NewEd25519Sign(2, 2, partList, adjustedShare2, publicKey, messageHex)

	// 执行简化的签名流程
	p1Step1, err := p1.SignStep1()
	if err != nil {
		return false
	}

	p2Step1, err := p2.SignStep1()
	if err != nil {
		return false
	}

	p1Step2, err := p1.SignStep2([]*tss.Message{p2Step1[1]})
	if err != nil {
		return false
	}

	p2Step2, err := p2.SignStep2([]*tss.Message{p1Step1[2]})
	if err != nil {
		return false
	}

	si_1, r, err := p1.SignStep3([]*tss.Message{p2Step2[1]})
	if err != nil {
		return false
	}

	si_2, _, err := p2.SignStep3([]*tss.Message{p1Step2[2]})
	if err != nil {
		return false
	}

	// 合并签名并验证
	s := new(big.Int).Add(si_1, si_2)
	signature := edwards.NewSignature(r, s)
	return signature.Verify(messageHash, publicKey)
}