package bip32

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/okx/threshold-lib/crypto/curves"
	"github.com/stretchr/testify/assert"
)

func TestEd25519TssKey(t *testing.T) {
	// 创建一个Ed25519测试密钥
	curve := edwards.Edwards()

	// 生成一个测试私钥
	privateKey := new(big.Int)
	privateKey.SetString("12345678901234567890", 10)

	// 计算对应的公钥点
	publicKeyPoint := curves.ScalarToPoint(curve, privateKey)

	// 测试链码
	chaincode := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// 创建Ed25519TssKey
	tssKey, err := NewEd25519TssKey(privateKey, publicKeyPoint, chaincode)
	assert.NoError(t, err)
	assert.NotNil(t, tssKey)

	// 验证初始值
	assert.Equal(t, privateKey, tssKey.ShareI())
	assert.Equal(t, publicKeyPoint, tssKey.PublicKey())
	assert.Equal(t, big.NewInt(0), tssKey.PrivateKeyOffset())

	// 测试链码
	expectedChaincode, _ := hex.DecodeString(chaincode)
	assert.Equal(t, expectedChaincode, tssKey.Chaincode())
}

func TestEd25519TssKeyChildDerivation(t *testing.T) {
	// 创建父密钥
	curve := edwards.Edwards()
	privateKey := new(big.Int)
	privateKey.SetString("12345678901234567890", 10)
	publicKeyPoint := curves.ScalarToPoint(curve, privateKey)
	chaincode := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	parentKey, err := NewEd25519TssKey(privateKey, publicKeyPoint, chaincode)
	assert.NoError(t, err)

	// 派生子密钥
	childIdx := uint32(0)
	childKey, err := parentKey.NewChildKey(childIdx)
	assert.NoError(t, err)
	assert.NotNil(t, childKey)

	// 验证子密钥与父密钥不同
	assert.NotEqual(t, parentKey.ShareI(), childKey.ShareI())
	assert.NotEqual(t, parentKey.PublicKey(), childKey.PublicKey())
	assert.NotEqual(t, parentKey.Chaincode(), childKey.Chaincode())

	// 验证偏移量不为零
	assert.NotEqual(t, big.NewInt(0), childKey.PrivateKeyOffset())
}

func TestEd25519TssKeyHardenedDerivation(t *testing.T) {
	// 创建父密钥
	curve := edwards.Edwards()
	privateKey := new(big.Int)
	privateKey.SetString("12345678901234567890", 10)
	publicKeyPoint := curves.ScalarToPoint(curve, privateKey)
	chaincode := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	parentKey, err := NewEd25519TssKey(privateKey, publicKeyPoint, chaincode)
	assert.NoError(t, err)

	// 尝试硬化派生（应该失败）
	hardenedIdx := uint32(0x80000000) // 2^31
	_, err = parentKey.NewChildKey(hardenedIdx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "hardened derivation is unsupported")
}

func TestEd25519TssKeyBatchDerivation(t *testing.T) {
	// 创建父密钥
	curve := edwards.Edwards()
	privateKey := new(big.Int)
	privateKey.SetString("12345678901234567890", 10)
	publicKeyPoint := curves.ScalarToPoint(curve, privateKey)
	chaincode := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	parentKey, err := NewEd25519TssKey(privateKey, publicKeyPoint, chaincode)
	assert.NoError(t, err)

	// 批量派生路径 m/0/1/2
	path := []uint32{0, 1, 2}
	derivedKey, err := parentKey.DeriveChildKeys(path)
	assert.NoError(t, err)
	assert.NotNil(t, derivedKey)

	// 验证与逐步派生的结果一致
	step1, err := parentKey.NewChildKey(0)
	assert.NoError(t, err)
	step2, err := step1.NewChildKey(1)
	assert.NoError(t, err)
	step3, err := step2.NewChildKey(2)
	assert.NoError(t, err)

	assert.Equal(t, step3.ShareI(), derivedKey.ShareI())
	assert.Equal(t, step3.PublicKey().X, derivedKey.PublicKey().X)
	assert.Equal(t, step3.PublicKey().Y, derivedKey.PublicKey().Y)
	assert.Equal(t, step3.PrivateKeyOffset(), derivedKey.PrivateKeyOffset())
}

func TestEd25519TssKeyValidation(t *testing.T) {
	// 测试无效参数
	curve := edwards.Edwards()
	privateKey := new(big.Int)
	privateKey.SetString("12345678901234567890", 10)
	publicKeyPoint := curves.ScalarToPoint(curve, privateKey)

	// 测试空链码
	_, err := NewEd25519TssKey(privateKey, publicKeyPoint, "")
	assert.Error(t, err)

	// 测试空公钥
	chaincode := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	_, err = NewEd25519TssKey(privateKey, nil, chaincode)
	assert.Error(t, err)

	// 测试无效链码格式
	_, err = NewEd25519TssKey(privateKey, publicKeyPoint, "invalid_hex")
	assert.Error(t, err)
}

func TestEd25519TssKeyPublicKeyConversion(t *testing.T) {
	// 创建测试密钥
	curve := edwards.Edwards()
	privateKey := new(big.Int)
	privateKey.SetString("12345678901234567890", 10)
	publicKeyPoint := curves.ScalarToPoint(curve, privateKey)
	chaincode := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	tssKey, err := NewEd25519TssKey(privateKey, publicKeyPoint, chaincode)
	assert.NoError(t, err)

	// 转换为Ed25519公钥
	ed25519PubKey := tssKey.ToEd25519PublicKey()
	assert.NotNil(t, ed25519PubKey)

	// 验证公钥坐标一致
	assert.Equal(t, publicKeyPoint.X, ed25519PubKey.GetX())
	assert.Equal(t, publicKeyPoint.Y, ed25519PubKey.GetY())
}

func TestEd25519PrivateKeyValidation(t *testing.T) {
	// 测试有效的32字节密钥
	validKey := make([]byte, 32)
	validKey[0] = 1 // 非零密钥
	err := validateEd25519PrivateKey(validKey)
	assert.NoError(t, err)

	// 测试无效长度
	invalidKey := make([]byte, 31)
	err = validateEd25519PrivateKey(invalidKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be 32 bytes")

	// 测试全零密钥
	zeroKey := make([]byte, 32)
	err = validateEd25519PrivateKey(zeroKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be all zeros")
}

func TestEd25519DerivationPath(t *testing.T) {
	// 创建测试密钥
	curve := edwards.Edwards()
	privateKey := new(big.Int)
	privateKey.SetString("12345678901234567890", 10)
	publicKeyPoint := curves.ScalarToPoint(curve, privateKey)
	chaincode := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	tssKey, err := NewEd25519TssKey(privateKey, publicKeyPoint, chaincode)
	assert.NoError(t, err)

	// 获取派生路径信息
	pathInfo := tssKey.GetDerivationPath()
	assert.Contains(t, pathInfo, "Ed25519 TSS Key")
	assert.Contains(t, pathInfo, "Offset")
}
