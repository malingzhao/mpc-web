package bip32

import (
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/okx/threshold-lib/crypto/curves"
)

var ed25519Label = []byte("Ed25519 key share derivation:\n")

// Ed25519TssKey 支持Ed25519的BIP32密钥派生
type Ed25519TssKey struct {
	shareI       *big.Int        // key share
	publicKey    *curves.ECPoint // publicKey
	chaincode    []byte
	offsetSonPri *big.Int // child private key share offset, accumulative
}

// NewEd25519TssKey 创建新的Ed25519 TSS密钥，shareI是可选的
func NewEd25519TssKey(shareI *big.Int, publicKey *curves.ECPoint, chaincode string) (*Ed25519TssKey, error) {
	chainBytes, err := hex.DecodeString(chaincode)
	if err != nil {
		return nil, err
	}
	if publicKey == nil || chaincode == "" {
		return nil, fmt.Errorf("parameter error")
	}

	// 验证是否为Ed25519曲线
	if !isEd25519Curve(publicKey.Curve) {
		return nil, fmt.Errorf("publicKey must be on Ed25519 curve")
	}

	tssKey := &Ed25519TssKey{
		shareI:       shareI,
		publicKey:    publicKey,
		chaincode:    chainBytes,
		offsetSonPri: big.NewInt(0),
	}
	return tssKey, nil
}

// NewChildKey 类似BIP32的非硬化派生，专门为Ed25519设计
func (tssKey *Ed25519TssKey) NewChildKey(childIdx uint32) (*Ed25519TssKey, error) {
	if childIdx >= uint32(0x80000000) { // 2^31
		return nil, fmt.Errorf("hardened derivation is unsupported")
	}

	curve := tssKey.publicKey.Curve
	intermediary, err := calEd25519PrivateOffset(tssKey.publicKey.X.Bytes(), tssKey.chaincode, childIdx)
	if err != nil {
		return nil, err
	}

	// 验证Ed25519密钥
	err = validateEd25519PrivateKey(intermediary[:32])
	if err != nil {
		return nil, err
	}

	offset := new(big.Int).SetBytes(intermediary[:32])

	// 对于Ed25519，我们需要确保offset在正确的范围内
	offset = new(big.Int).Mod(offset, curve.Params().N)

	point := curves.ScalarToPoint(curve, offset)
	ecPoint, err := tssKey.publicKey.Add(point)
	if err != nil {
		return nil, err
	}

	shareI := tssKey.shareI
	if shareI != nil {
		shareI = new(big.Int).Add(shareI, offset)
		shareI = new(big.Int).Mod(shareI, curve.Params().N)
	}

	offsetSonPri := new(big.Int).Add(tssKey.offsetSonPri, offset)
	offsetSonPri = new(big.Int).Mod(offsetSonPri, curve.Params().N)

	tss := &Ed25519TssKey{
		shareI:       shareI,
		publicKey:    ecPoint,
		chaincode:    intermediary[32:],
		offsetSonPri: offsetSonPri,
	}
	return tss, nil
}

// PrivateKeyOffset 子密钥份额偏移量，累积的
func (tssKey *Ed25519TssKey) PrivateKeyOffset() *big.Int {
	return tssKey.offsetSonPri
}

// ShareI 子密钥份额
func (tssKey *Ed25519TssKey) ShareI() *big.Int {
	return tssKey.shareI
}

// PublicKey 子公钥
func (tssKey *Ed25519TssKey) PublicKey() *curves.ECPoint {
	return tssKey.publicKey
}

// Chaincode 返回链码
func (tssKey *Ed25519TssKey) Chaincode() []byte {
	return tssKey.chaincode
}

// calEd25519PrivateOffset 计算Ed25519私钥偏移量
// HMAC-SHA512(ed25519Label | chaincode | publicKey | childIdx)
func calEd25519PrivateOffset(publicKey, chaincode []byte, childIdx uint32) ([]byte, error) {
	hash := hmac.New(sha512.New, ed25519Label)
	var data []byte
	data = append(data, chaincode...)
	data = append(data, publicKey...)
	data = append(data, uint32Bytes(childIdx)...)
	_, err := hash.Write(data)
	if err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}

// validateEd25519PrivateKey 验证Ed25519私钥
func validateEd25519PrivateKey(key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("Ed25519 private key must be 32 bytes")
	}

	// 检查是否为全零
	allZero := true
	for _, b := range key {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return fmt.Errorf("Ed25519 private key cannot be all zeros")
	}

	// Ed25519的私钥验证相对宽松，只要不是全零且长度正确即可
	// Ed25519会在内部处理私钥的范围问题
	return nil
}

// isEd25519Curve 检查是否为Ed25519曲线
func isEd25519Curve(curve interface{}) bool {
	if c, ok := curve.(elliptic.Curve); ok {
		return curves.GetCurveName(c) == curves.Ed25519
	}
	return false
}

// DeriveChildKeys 批量派生子密钥
func (tssKey *Ed25519TssKey) DeriveChildKeys(path []uint32) (*Ed25519TssKey, error) {
	current := tssKey
	var err error

	for _, childIdx := range path {
		current, err = current.NewChildKey(childIdx)
		if err != nil {
			return nil, fmt.Errorf("failed to derive child key at index %d: %v", childIdx, err)
		}
	}

	return current, nil
}

// GetDerivationPath 获取派生路径信息
func (tssKey *Ed25519TssKey) GetDerivationPath() string {
	offsetStr := tssKey.offsetSonPri.String()
	if len(offsetStr) > 20 {
		offsetStr = offsetStr[:20] + "..."
	}
	return fmt.Sprintf("Ed25519 TSS Key - Offset: %s", offsetStr)
}

// ToEd25519PublicKey 转换为Ed25519公钥对象
func (tssKey *Ed25519TssKey) ToEd25519PublicKey() *edwards.PublicKey {
	return edwards.NewPublicKey(tssKey.publicKey.X, tssKey.publicKey.Y)
}
