package zeroex

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestDecodeERC20AssetData(t *testing.T) {
	assetData := common.Hex2Bytes("f47261b000000000000000000000000038ae374ecf4db50b0ff37125b591a04997106a32")

	d, err := NewAssetDataDecoder()
	if err != nil {
		t.Fatal(err.Error())
	}
	decodedAssetData, err := d.Decode(assetData)
	if err != nil {
		t.Fatal(err.Error())
	}

	expectedAssetData := ERC20AssetData{
		Address: common.HexToAddress("0x38ae374ecf4db50b0ff37125b591a04997106a32"),
	}
	actualDecodedAssetData := decodedAssetData.(ERC20AssetData)
	assert.Equal(t, expectedAssetData, actualDecodedAssetData, "ERC20 Asset Data properly decoded")
}

func TestDecodeERC721AssetData(t *testing.T) {
	assetData := common.Hex2Bytes("025717920000000000000000000000001dc4c1cefef38a777b15aa20260a54e584b16c480000000000000000000000000000000000000000000000000000000000000001")

	d, err := NewAssetDataDecoder()
	if err != nil {
		t.Fatal(err.Error())
	}
	decodedAssetData, err := d.Decode(assetData)
	if err != nil {
		t.Fatal(err.Error())
	}

	expectedDecodedAssetData := ERC721AssetData{
		Address: common.HexToAddress("0x1dC4c1cEFEF38a777b15aA20260a54E584b16C48"),
		TokenId: big.NewInt(1),
	}
	actualDecodedAssetData := decodedAssetData.(ERC721AssetData)
	assert.Equal(t, expectedDecodedAssetData, actualDecodedAssetData, "ERC721 Asset Data properly decoded")
}
func TestDecodeMultiAssetData(t *testing.T) {
	assetData := common.Hex2Bytes("94cfcdd7000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000c000000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000046000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000120000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000c000000000000000000000000000000000000000000000000000000000000001400000000000000000000000000000000000000000000000000000000000000024f47261b00000000000000000000000001dc4c1cefef38a777b15aa20260a54e584b16c48000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000044025717920000000000000000000000001dc4c1cefef38a777b15aa20260a54e584b16c480000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000204a7cb5fb70000000000000000000000001dc4c1cefef38a777b15aa20260a54e584b16c480000000000000000000000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001800000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000006400000000000000000000000000000000000000000000000000000000000003e90000000000000000000000000000000000000000000000000000000000002711000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000000000000000000000000000000000000c800000000000000000000000000000000000000000000000000000000000007d10000000000000000000000000000000000000000000000000000000000004e210000000000000000000000000000000000000000000000000000000000000044025717920000000000000000000000001dc4c1cefef38a777b15aa20260a54e584b16c4800000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")

	d, err := NewAssetDataDecoder()
	if err != nil {
		t.Fatal(err.Error())
	}
	decodedAssetData, err := d.Decode(assetData)
	if err != nil {
		t.Fatal(err.Error())
	}

	nestedAssetData := [][]byte{
		common.Hex2Bytes("f47261b00000000000000000000000001dc4c1cefef38a777b15aa20260a54e584b16c48"),
		common.Hex2Bytes("025717920000000000000000000000001dc4c1cefef38a777b15aa20260a54e584b16c480000000000000000000000000000000000000000000000000000000000000001"),
		common.Hex2Bytes("a7cb5fb70000000000000000000000001dc4c1cefef38a777b15aa20260a54e584b16c480000000000000000000000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001800000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000006400000000000000000000000000000000000000000000000000000000000003e90000000000000000000000000000000000000000000000000000000000002711000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000000000000000000000000000000000000c800000000000000000000000000000000000000000000000000000000000007d10000000000000000000000000000000000000000000000000000000000004e210000000000000000000000000000000000000000000000000000000000000044025717920000000000000000000000001dc4c1cefef38a777b15aa20260a54e584b16c48000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000"),
	}
	expectedDecodedAssetData := MultiAssetData{
		Amounts:         []*big.Int{big.NewInt(70), big.NewInt(1), big.NewInt(18)},
		NestedAssetData: nestedAssetData,
	}
	actualDecodedAssetData := decodedAssetData.(MultiAssetData)
	assert.Equal(t, expectedDecodedAssetData, actualDecodedAssetData, "Multi Asset Data properly decoded")
}
