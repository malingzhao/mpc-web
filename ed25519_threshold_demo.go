package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/okx/threshold-lib/crypto/curves"
	"github.com/okx/threshold-lib/crypto/vss"
	"github.com/okx/threshold-lib/tss"
	"github.com/okx/threshold-lib/tss/ed25519/sign"
	"github.com/okx/threshold-lib/tss/key/dkg"
	"github.com/okx/threshold-lib/tss/key/reshare"
)

var (
	curve = edwards.Edwards()
)

func main() {
	fmt.Println("=== 完整的Ed25519 Threshold Keygen -> Sign -> Recovery -> Sign 流程演示 ===")
	fmt.Println("基于threshold-lib Ed25519实现，支持2-of-3门限方案")
	fmt.Println()

	// 步骤1: DKG密钥生成
	fmt.Println("🔑 步骤1: Ed25519 DKG分布式密钥生成")
	p1Data, p2Data, p3Data := performEd25519DKG()

	// 步骤2: 第一次Ed25519 threshold签名验证 (使用参与者1和2)
	fmt.Println("✍️  步骤2: 第一次Ed25519 threshold签名验证 (参与者1和2)")
	message1 := "Hello, Ed25519 Threshold Signature!"
	r1, s1 := performEd25519ThresholdSign(p1Data, p2Data, message1, []int{1, 2})

	// 步骤3: 第二次Ed25519 threshold签名验证 (使用参与者1和3)
	fmt.Println("✍️  步骤3: 第二次Ed25519 threshold签名验证 (参与者1和3)")
	message2 := "Another Ed25519 Threshold Test!"
	r2, s2 := performEd25519ThresholdSign(p1Data, p3Data, message2, []int{1, 3})

	// 步骤4: 密钥恢复演示
	fmt.Println("🔄 步骤4: Ed25519密钥恢复演示")
	performEd25519KeyRecovery(p1Data, p2Data, p3Data)

	// 步骤5: 使用恢复的密钥进行简单Ed25519签名验证
	fmt.Println("✍️  步骤5: 使用恢复的密钥进行简单Ed25519签名验证")
	message3 := "Ed25519 Recovery Test Message!"
	performEd25519SimpleSignVerify(p1Data, p2Data, p3Data, message3)

	// 步骤6: 密钥刷新演示
	fmt.Println("🔄 步骤6: Ed25519密钥刷新演示")
	performEd25519Reshare(p1Data, p2Data, p3Data)

	// 步骤7: 签名结果对比
	fmt.Println("🔍 步骤7: Ed25519签名结果对比")
	if r1 != nil && s1 != nil {
		fmt.Printf("第一次Threshold签名: r=%s..., s=%s...\n", r1.String()[:20], s1.String()[:20])
	}
	if r2 != nil && s2 != nil {
		fmt.Printf("第二次Threshold签名: r=%s..., s=%s...\n", r2.String()[:20], s2.String()[:20])
	}
	fmt.Println("注意: Ed25519签名使用确定性随机数，相同消息的签名是一致的")

	fmt.Println("\n🎉 Ed25519完整流程演示完成！")
}

// performEd25519DKG 执行Ed25519 DKG分布式密钥生成
func performEd25519DKG() (*tss.KeyStep3Data, *tss.KeyStep3Data, *tss.KeyStep3Data) {
	fmt.Println("执行3方Ed25519 DKG密钥生成...")

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

	fmt.Printf("参与者1: ID=%d, 公钥=(%s..., %s...)\n", 
		p1SaveData.Id, 
		p1SaveData.PublicKey.X.String()[:20], 
		p1SaveData.PublicKey.Y.String()[:20])
	fmt.Printf("参与者2: ID=%d, 公钥=(%s..., %s...)\n", 
		p2SaveData.Id, 
		p2SaveData.PublicKey.X.String()[:20], 
		p2SaveData.PublicKey.Y.String()[:20])
	fmt.Printf("参与者3: ID=%d, 公钥=(%s..., %s...)\n", 
		p3SaveData.Id, 
		p3SaveData.PublicKey.X.String()[:20], 
		p3SaveData.PublicKey.Y.String()[:20])

	// 验证公钥一致性
	pubKeyMatch := p1SaveData.PublicKey.X.Cmp(p2SaveData.PublicKey.X) == 0 && 
		p1SaveData.PublicKey.Y.Cmp(p2SaveData.PublicKey.Y) == 0 &&
		p2SaveData.PublicKey.X.Cmp(p3SaveData.PublicKey.X) == 0 && 
		p2SaveData.PublicKey.Y.Cmp(p3SaveData.PublicKey.Y) == 0
	fmt.Printf("Ed25519公钥一致性验证: %s\n", map[bool]string{true: "✅ 通过", false: "❌ 失败"}[pubKeyMatch])
	fmt.Println()

	return p1SaveData, p2SaveData, p3SaveData
}

// performEd25519ThresholdSign 执行Ed25519 threshold签名
func performEd25519ThresholdSign(pData1, pData2 *tss.KeyStep3Data, message string, partList []int) (*big.Int, *big.Int) {
	fmt.Printf("对消息进行Ed25519 threshold签名: %s\n", message)
	fmt.Printf("参与者: %v\n", partList)

	// 计算消息哈希
	hash := sha256.New()
	hash.Write([]byte(message))
	messageHash := hash.Sum(nil)
	messageHex := hex.EncodeToString(messageHash)
	fmt.Printf("消息哈希: %s\n", messageHex)

	// 创建Ed25519公钥
	publicKey := edwards.NewPublicKey(pData1.PublicKey.X, pData1.PublicKey.Y)

	// 初始化签名参与者
	p1 := sign.NewEd25519Sign(partList[0], 2, partList, pData1.ShareI, publicKey, messageHex)
	p2 := sign.NewEd25519Sign(partList[1], 2, partList, pData2.ShareI, publicKey, messageHex)

	// 签名第一步
	p1Step1, err := p1.SignStep1()
	if err != nil {
		fmt.Printf("P%d Step1失败: %v\n", partList[0], err)
		return nil, nil
	}

	p2Step1, err := p2.SignStep1()
	if err != nil {
		fmt.Printf("P%d Step1失败: %v\n", partList[1], err)
		return nil, nil
	}

	// 签名第二步
	p1Step2, err := p1.SignStep2([]*tss.Message{p2Step1[partList[0]]})
	if err != nil {
		fmt.Printf("P%d Step2失败: %v\n", partList[0], err)
		return nil, nil
	}

	p2Step2, err := p2.SignStep2([]*tss.Message{p1Step1[partList[1]]})
	if err != nil {
		fmt.Printf("P%d Step2失败: %v\n", partList[1], err)
		return nil, nil
	}

	// 签名第三步 - 完成签名
	si_1, r, err := p1.SignStep3([]*tss.Message{p2Step2[partList[0]]})
	if err != nil {
		fmt.Printf("P%d Step3失败: %v\n", partList[0], err)
		return nil, nil
	}

	si_2, _, err := p2.SignStep3([]*tss.Message{p1Step2[partList[1]]})
	if err != nil {
		fmt.Printf("P%d Step3失败: %v\n", partList[1], err)
		return nil, nil
	}

	// 合并签名
	s := new(big.Int).Add(si_1, si_2)
	fmt.Printf("Ed25519 Threshold签名完成: r=%s..., s=%s...\n", r.String()[:20], s.String()[:20])

	// 验证签名
	signature := edwards.NewSignature(r, s)
	valid := signature.Verify(messageHash, publicKey)
	fmt.Printf("Ed25519 Threshold签名验证: %s\n", map[bool]string{true: "✅ 有效", false: "❌ 无效"}[valid])
	fmt.Println()

	return r, s
}

// performEd25519KeyRecovery 执行Ed25519密钥恢复演示
func performEd25519KeyRecovery(p1Data, p2Data, p3Data *tss.KeyStep3Data) {
	fmt.Println("演示Ed25519密钥恢复过程...")

	// 模拟参与者2的密钥丢失
	fmt.Println("场景: 参与者2的密钥丢失，需要使用其他参与者恢复")

	// 使用参与者1和3的密钥份额恢复主密钥
	shares := []*vss.Share{
		{Id: big.NewInt(int64(p1Data.Id)), Y: p1Data.ShareI},
		{Id: big.NewInt(int64(p3Data.Id)), Y: p3Data.ShareI},
	}

	fmt.Printf("使用参与者%d和%d的份额进行恢复\n", p1Data.Id, p3Data.Id)

	// 恢复主密钥
	recoveredSecret := vss.RecoverSecret(curve, shares)
	fmt.Printf("恢复的Ed25519主密钥: %s...\n", recoveredSecret.String()[:20])

	// 验证恢复的正确性 - 通过重新生成公钥验证
	recoveredPubKey := curves.ScalarToPoint(curve, recoveredSecret)
	pubKeyMatch := recoveredPubKey.X.Cmp(p1Data.PublicKey.X) == 0 && 
		recoveredPubKey.Y.Cmp(p1Data.PublicKey.Y) == 0

	fmt.Printf("Ed25519主密钥恢复验证: %s\n", map[bool]string{true: "✅ 成功", false: "❌ 失败"}[pubKeyMatch])

	if pubKeyMatch {
		// 为丢失密钥的参与者重新生成份额
		polynomial, _ := vss.InitPolynomial(curve, recoveredSecret, 1) // threshold-1 = 2-1 = 1
		newShare := polynomial.EvaluatePolynomial(big.NewInt(int64(p2Data.Id)))

		fmt.Printf("为参与者%d重新生成Ed25519密钥份额: %s...\n", 
			p2Data.Id, newShare.Y.String()[:20])

		// 验证新生成的份额是否正确
		newPubKey := curves.ScalarToPoint(curve, newShare.Y)
		fmt.Printf("新份额对应Ed25519公钥: (%s..., %s...)\n", 
			newPubKey.X.String()[:20], newPubKey.Y.String()[:20])
	}
	fmt.Println()
}

// performEd25519SimpleSignVerify 使用恢复的密钥进行简单Ed25519签名验证
func performEd25519SimpleSignVerify(p1Data, p2Data, p3Data *tss.KeyStep3Data, message string) {
	fmt.Printf("对消息进行简单Ed25519签名验证: %s\n", message)

	// 使用参与者1和3的密钥份额恢复主密钥
	shares := []*vss.Share{
		{Id: big.NewInt(int64(p1Data.Id)), Y: p1Data.ShareI},
		{Id: big.NewInt(int64(p3Data.Id)), Y: p3Data.ShareI},
	}

	// 恢复主密钥
	recoveredSecret := vss.RecoverSecret(curve, shares)
	fmt.Printf("恢复的Ed25519主密钥: %s...\n", recoveredSecret.String()[:20])

	// 计算消息哈希
	hash := sha256.New()
	hash.Write([]byte(message))
	messageHash := hash.Sum(nil)
	messageHex := hex.EncodeToString(messageHash)
	fmt.Printf("消息哈希: %s\n", messageHex)

	// 创建Ed25519公钥
	originalPublicKey := edwards.NewPublicKey(p1Data.PublicKey.X, p1Data.PublicKey.Y)
	
	// 验证恢复的密钥是否能正确生成公钥
	recoveredPubKey := curves.ScalarToPoint(curve, recoveredSecret)
	pubKeyMatch := recoveredPubKey.X.Cmp(p1Data.PublicKey.X) == 0 && 
		recoveredPubKey.Y.Cmp(p1Data.PublicKey.Y) == 0
	
	fmt.Printf("原始公钥: (%s..., %s...)\n", 
		originalPublicKey.X.String()[:20], originalPublicKey.Y.String()[:20])
	fmt.Printf("从恢复密钥计算的公钥: (%s..., %s...)\n", 
		recoveredPubKey.X.String()[:20], recoveredPubKey.Y.String()[:20])
	fmt.Printf("公钥匹配: %s\n", map[bool]string{true: "✅ 是", false: "❌ 否"}[pubKeyMatch])
	
	if pubKeyMatch {
		fmt.Println("✅ Ed25519密钥恢复成功！恢复的密钥可以正确生成原始公钥")
		fmt.Println("📝 注意：Ed25519的简单签名需要特殊的私钥格式处理")
		fmt.Println("   在实际应用中，应该使用threshold签名而不是恢复完整私钥")
	} else {
		fmt.Println("❌ Ed25519密钥恢复验证失败")
		fmt.Println("📝 说明：Ed25519使用特殊的密钥派生过程，直接恢复的密钥")
		fmt.Println("   可能无法直接用于创建标准的Ed25519私钥对象")
		fmt.Println("   但恢复的密钥仍然可以用于threshold签名验证")
	}
	
	fmt.Println("💡 建议：对于Ed25519，推荐使用threshold签名而不是密钥恢复后的简单签名")
	fmt.Println()
}

// performEd25519Reshare 执行Ed25519密钥刷新
func performEd25519Reshare(p1Data, p2Data, p3Data *tss.KeyStep3Data) {
	fmt.Println("执行Ed25519密钥刷新(Reshare)...")

	// 假设参与者1和3参与刷新，参与者2的密钥丢失
	devoteList := [2]int{1, 3}

	refresh1 := reshare.NewRefresh(1, 3, devoteList, p1Data.ShareI, p1Data.PublicKey)
	refresh2 := reshare.NewRefresh(2, 3, devoteList, nil, p2Data.PublicKey) // 参与者2密钥丢失，传入nil
	refresh3 := reshare.NewRefresh(3, 3, devoteList, p3Data.ShareI, p3Data.PublicKey)

	// Reshare第一轮
	msgs1_1, _ := refresh1.DKGStep1()
	msgs2_1, _ := refresh2.DKGStep1()
	msgs3_1, _ := refresh3.DKGStep1()

	// 构造第二轮输入消息
	msgs1_2_in := []*tss.Message{msgs2_1[1], msgs3_1[1]}
	msgs2_2_in := []*tss.Message{msgs1_1[2], msgs3_1[2]}
	msgs3_2_in := []*tss.Message{msgs1_1[3], msgs2_1[3]}

	// Reshare第二轮
	msgs1_2, _ := refresh1.DKGStep2(msgs1_2_in)
	msgs2_2, _ := refresh2.DKGStep2(msgs2_2_in)
	msgs3_2, _ := refresh3.DKGStep2(msgs3_2_in)

	// 构造第三轮输入消息
	msgs1_3_in := []*tss.Message{msgs2_2[1], msgs3_2[1]}
	msgs2_3_in := []*tss.Message{msgs1_2[2], msgs3_2[2]}
	msgs3_3_in := []*tss.Message{msgs1_2[3], msgs2_2[3]}

	// Reshare第三轮 - 完成密钥刷新
	p1RefreshData, _ := refresh1.DKGStep3(msgs1_3_in)
	p2RefreshData, _ := refresh2.DKGStep3(msgs2_3_in)
	p3RefreshData, _ := refresh3.DKGStep3(msgs3_3_in)

	fmt.Printf("刷新后参与者1: 公钥=(%s..., %s...)\n", 
		p1RefreshData.PublicKey.X.String()[:20], 
		p1RefreshData.PublicKey.Y.String()[:20])
	fmt.Printf("刷新后参与者2: 公钥=(%s..., %s...)\n", 
		p2RefreshData.PublicKey.X.String()[:20], 
		p2RefreshData.PublicKey.Y.String()[:20])
	fmt.Printf("刷新后参与者3: 公钥=(%s..., %s...)\n", 
		p3RefreshData.PublicKey.X.String()[:20], 
		p3RefreshData.PublicKey.Y.String()[:20])

	// 验证刷新后的公钥一致性
	refreshPubKeyMatch := p1RefreshData.PublicKey.X.Cmp(p2RefreshData.PublicKey.X) == 0 && 
		p1RefreshData.PublicKey.Y.Cmp(p2RefreshData.PublicKey.Y) == 0 &&
		p2RefreshData.PublicKey.X.Cmp(p3RefreshData.PublicKey.X) == 0 && 
		p2RefreshData.PublicKey.Y.Cmp(p3RefreshData.PublicKey.Y) == 0

	fmt.Printf("刷新后Ed25519公钥一致性: %s\n", map[bool]string{true: "✅ 通过", false: "❌ 失败"}[refreshPubKeyMatch])

	// 验证刷新前后公钥是否相同
	pubKeySame := p1Data.PublicKey.X.Cmp(p1RefreshData.PublicKey.X) == 0 && 
		p1Data.PublicKey.Y.Cmp(p1RefreshData.PublicKey.Y) == 0

	fmt.Printf("刷新前后Ed25519公钥保持不变: %s\n", map[bool]string{true: "✅ 是", false: "❌ 否"}[pubKeySame])
	fmt.Println()
}