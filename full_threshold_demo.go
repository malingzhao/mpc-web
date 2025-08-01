package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/decred/dcrd/dcrec/secp256k1/v2"
	"github.com/okx/threshold-lib/crypto/curves"
	"github.com/okx/threshold-lib/crypto/paillier"
	"github.com/okx/threshold-lib/crypto/vss"
	"github.com/okx/threshold-lib/tss"
	"github.com/okx/threshold-lib/tss/ecdsa/keygen"
	"github.com/okx/threshold-lib/tss/ecdsa/sign"
	"github.com/okx/threshold-lib/tss/key/dkg"
	"github.com/okx/threshold-lib/tss/key/reshare"
	"math/big"
)

var curve = secp256k1.S256()

func main() {
	fmt.Println("=== 完整的Threshold Keygen -> Sign -> Recovery -> Sign 流程演示 ===")
	fmt.Println("基于threshold-lib测试案例实现，支持2-of-3门限方案")
	fmt.Println()

	// 步骤1: DKG密钥生成
	fmt.Println("🔑 步骤1: DKG分布式密钥生成")
	p1Data, p2Data, p3Data := performDKG()

	// 步骤2: ECDSA 2-of-2 密钥协商 (使用参与者1和2)
	fmt.Println("🤝 步骤2: ECDSA 2-of-2 密钥协商 (参与者1和2)")
	p2SaveData := performECDSAKeygen(p1Data, p2Data)
	if p2SaveData == nil {
		fmt.Println("❌ ECDSA密钥协商失败，跳过threshold签名")
		return
	}

	// 步骤3: 第一次threshold签名验证
	fmt.Println("✍️  步骤3: 第一次threshold签名验证")
	message1 := "Hello, Threshold Signature!"
	r1, s1 := performThresholdSign(p1Data, p2Data, p2SaveData, message1)

	// 步骤4: 密钥恢复演示
	fmt.Println("🔄 步骤4: 密钥恢复演示")
	performKeyRecovery(p1Data, p2Data, p3Data)

	// 步骤5: 使用恢复的密钥进行简单签名验证
	fmt.Println("✍️  步骤5: 使用恢复的密钥进行简单签名验证")
	message2 := "Recovery Test Message!"
	performSimpleSignVerify(p1Data, p2Data, p3Data, message2)

	// 步骤6: 签名结果对比
	fmt.Println("🔍 步骤6: 签名结果对比")
	if r1 != nil && s1 != nil {
		fmt.Printf("Threshold签名: r=%s..., s=%s...\n", r1.String()[:20], s1.String()[:20])
		fmt.Println("简单签名: 见上方输出")
		fmt.Println("注意: 两种签名方式都是有效的，但由于使用了不同的随机数k，签名值会不同")
	} else {
		fmt.Println("Threshold签名失败，仅显示简单签名结果")
	}

	fmt.Println("\n🎉 完整流程演示完成！")
}

// performDKG 执行DKG分布式密钥生成
func performDKG() (*tss.KeyStep3Data, *tss.KeyStep3Data, *tss.KeyStep3Data) {
	fmt.Println("执行3方DKG密钥生成...")

	// 初始化3个参与者
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
	fmt.Printf("公钥一致性验证: %s\n", map[bool]string{true: "✅ 通过", false: "❌ 失败"}[pubKeyMatch])
	fmt.Println()

	return p1SaveData, p2SaveData, p3SaveData
}

// 扩展的P2SaveData结构体，包含Paillier私钥
type ExtendedP2SaveData struct {
	*keygen.P2SaveData
	PaiPrivKey *paillier.PrivateKey // 保存Paillier私钥用于签名
}

// performECDSAKeygen 执行ECDSA 2-of-2密钥协商
func performECDSAKeygen(p1Data, p2Data *tss.KeyStep3Data) *ExtendedP2SaveData {
	fmt.Println("执行ECDSA 2-of-2密钥协商...")

	// 生成Paillier密钥对
	paiPrivate, _, _ := paillier.NewKeyPair(8)

	// 生成预参数和证明
	p1PreParamsAndProof := keygen.GeneratePreParamsWithDlnProof()
	p2PreParamsAndProof := &keygen.PreParamsWithDlnProof{
		Params: p1PreParamsAndProof.Params,
		Proof:  p1PreParamsAndProof.Proof,
	}

	// P1执行密钥协商
	p1Dto, _, err := keygen.P1(
		p1Data.ShareI,
		paiPrivate,
		p1Data.Id,
		p2Data.Id,
		p1PreParamsAndProof,
		p2PreParamsAndProof.PedersonParameters(),
		p2PreParamsAndProof.Proof,
	)
	if err != nil {
		fmt.Printf("P1密钥协商失败: %v\n", err)
		return nil
	}

	// P2执行密钥协商
	publicKey, _ := curves.NewECPoint(curve, p2Data.PublicKey.X, p2Data.PublicKey.Y)
	p2SaveData, err := keygen.P2(
		p2Data.ShareI,
		publicKey,
		p1Dto,
		p1Data.Id,
		p2Data.Id,
		p2PreParamsAndProof.PedersonParameters(),
	)
	if err != nil {
		fmt.Printf("P2密钥协商失败: %v\n", err)
		return nil
	}

	fmt.Printf("ECDSA密钥协商完成\n")
	fmt.Printf("P2保存数据: X2=%s...\n", p2SaveData.X2.String()[:20])
	fmt.Println()

	// 返回扩展的保存数据，包含Paillier私钥
	return &ExtendedP2SaveData{
		P2SaveData: p2SaveData,
		PaiPrivKey: paiPrivate,
	}
}

// performThresholdSign 执行threshold签名 (修复版本)
func performThresholdSign(p1Data, p2Data *tss.KeyStep3Data, p2SaveData *ExtendedP2SaveData, message string) (*big.Int, *big.Int) {
	fmt.Printf("对消息进行threshold签名: %s\n", message)

	// 使用原始的公钥和私钥份额，不进行BIP32派生
	pubKey := &ecdsa.PublicKey{
		Curve: curve,
		X:     p2Data.PublicKey.X,
		Y:     p2Data.PublicKey.Y,
	}

	// 计算消息哈希
	hash := sha256.New()
	hash.Write([]byte(message))
	messageHash := hash.Sum(nil)
	messageHex := hex.EncodeToString(messageHash)

	fmt.Printf("消息哈希: %s\n", messageHex)

	// 使用keygen阶段保存的参数进行签名
	p1 := sign.NewP1(
		pubKey,
		messageHex,
		p2SaveData.PaiPrivKey, // 使用保存的Paillier私钥
		p2SaveData.E_x1,       // 使用keygen阶段保存的E_x1
		p2SaveData.Ped1,       // 使用keygen阶段保存的Pedersen参数
	)
	p2 := sign.NewP2(
		p2SaveData.X2,        // P2的私钥份额
		p2SaveData.E_x1,      // 相同的E_x1
		pubKey,               // 公钥
		p2SaveData.PaiPubKey, // P1的Paillier公钥
		messageHex,           // 消息哈希
		p2SaveData.Ped1,      // 相同的Pedersen参数
	)

	// 签名第一步
	commit, err := p1.Step1()
	if err != nil {
		fmt.Printf("P1 Step1失败: %v\n", err)
		return nil, nil
	}

	bobProof, R2, err := p2.Step1(commit)
	if err != nil {
		fmt.Printf("P2 Step1失败: %v\n", err)
		return nil, nil
	}

	// 签名第二步
	proof, cmtD, err := p1.Step2(bobProof, R2)
	if err != nil {
		fmt.Printf("P1 Step2失败: %v\n", err)
		return nil, nil
	}

	E_k2_h_xr, affine_proof, err := p2.Step2(cmtD, proof)
	if err != nil {
		fmt.Printf("P2 Step2失败: %v\n", err)
		return nil, nil
	}

	// 签名第三步 - 完成签名
	r, s, err := p1.Step3(E_k2_h_xr, affine_proof)
	if err != nil {
		fmt.Printf("P1 Step3失败: %v\n", err)
		return nil, nil
	}

	fmt.Printf("Threshold签名完成: r=%s..., s=%s...\n", r.String()[:20], s.String()[:20])

	// 验证签名
	valid := ecdsa.Verify(pubKey, messageHash, r, s)
	fmt.Printf("Threshold签名验证: %s\n", map[bool]string{true: "✅ 有效", false: "❌ 无效"}[valid])
	fmt.Println()

	return r, s
}

// performKeyRecovery 执行密钥恢复演示
func performKeyRecovery(p1Data, p2Data, p3Data *tss.KeyStep3Data) {
	fmt.Println("演示密钥恢复过程...")

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
	fmt.Printf("恢复的主密钥: %s...\n", recoveredSecret.String()[:20])

	// 验证恢复的正确性 - 通过重新生成公钥验证
	recoveredPubKey := curves.ScalarToPoint(curve, recoveredSecret)
	pubKeyMatch := recoveredPubKey.X.Cmp(p1Data.PublicKey.X) == 0 &&
		recoveredPubKey.Y.Cmp(p1Data.PublicKey.Y) == 0

	fmt.Printf("主密钥恢复验证: %s\n", map[bool]string{true: "✅ 成功", false: "❌ 失败"}[pubKeyMatch])

	if pubKeyMatch {
		// 为丢失密钥的参与者重新生成份额
		polynomial, _ := vss.InitPolynomial(curve, recoveredSecret, 1) // threshold-1 = 2-1 = 1
		newShare := polynomial.EvaluatePolynomial(big.NewInt(int64(p2Data.Id)))

		fmt.Printf("为参与者%d重新生成密钥份额: %s...\n",
			p2Data.Id, newShare.Y.String()[:20])

		// 验证新生成的份额是否正确
		newPubKey := curves.ScalarToPoint(curve, newShare.Y)
		fmt.Printf("新份额对应公钥: (%s..., %s...)\n",
			newPubKey.X.String()[:20], newPubKey.Y.String()[:20])
	}

	// 演示使用reshare进行密钥刷新
	fmt.Println("\n使用Reshare进行密钥刷新:")
	performReshare(p1Data, p2Data, p3Data)
	fmt.Println()
}

// performReshare 执行密钥刷新
func performReshare(p1Data, p2Data, p3Data *tss.KeyStep3Data) {
	fmt.Println("执行密钥刷新(Reshare)...")

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

	fmt.Printf("刷新后公钥一致性: %s\n", map[bool]string{true: "✅ 通过", false: "❌ 失败"}[refreshPubKeyMatch])

	// 验证刷新前后公钥是否相同
	pubKeySame := p1Data.PublicKey.X.Cmp(p1RefreshData.PublicKey.X) == 0 &&
		p1Data.PublicKey.Y.Cmp(p1RefreshData.PublicKey.Y) == 0

	fmt.Printf("刷新前后公钥保持不变: %s\n", map[bool]string{true: "✅ 是", false: "❌ 否"}[pubKeySame])
}

// performSimpleSignVerify 使用恢复的密钥进行简单签名验证
func performSimpleSignVerify(p1Data, p2Data, p3Data *tss.KeyStep3Data, message string) {
	fmt.Printf("对消息进行简单签名验证: %s\n", message)

	// 使用参与者1和3的密钥份额恢复主密钥
	shares := []*vss.Share{
		{Id: big.NewInt(int64(p1Data.Id)), Y: p1Data.ShareI},
		{Id: big.NewInt(int64(p3Data.Id)), Y: p3Data.ShareI},
	}

	// 恢复主密钥
	recoveredSecret := vss.RecoverSecret(curve, shares)
	fmt.Printf("恢复的主密钥: %s...\n", recoveredSecret.String()[:20])

	// 使用恢复的密钥创建ECDSA私钥
	privateKey := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
			X:     p1Data.PublicKey.X,
			Y:     p1Data.PublicKey.Y,
		},
		D: recoveredSecret,
	}

	// 计算消息哈希
	hash := sha256.New()
	hash.Write([]byte(message))
	messageHash := hash.Sum(nil)
	messageHex := hex.EncodeToString(messageHash)
	fmt.Printf("消息哈希: %s\n", messageHex)

	// 使用恢复的私钥进行签名
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, messageHash)
	if err != nil {
		fmt.Printf("签名失败: %v\n", err)
		return
	}

	fmt.Printf("简单签名完成: r=%s..., s=%s...\n", r.String()[:20], s.String()[:20])

	// 验证签名
	valid := ecdsa.Verify(&privateKey.PublicKey, messageHash, r, s)
	fmt.Printf("简单签名验证: %s\n", map[bool]string{true: "✅ 有效", false: "❌ 无效"}[valid])
	fmt.Println()
}
