package zeroex

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/0xProject/0x-mesh/ethereum"
	"github.com/0xProject/0x-mesh/ethereum/wrappers"
	"github.com/0xProject/0x-mesh/zeroex/orderwatch/decoder"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	signer "github.com/ethereum/go-ethereum/signer/core"
	"golang.org/x/crypto/sha3"
)

// Order represents an unsigned 0x order
type Order struct {
	MakerAddress          common.Address `json:"makerAddress"`
	MakerAssetData        []byte         `json:"makerAssetData"`
	MakerFeeAssetData     []byte         `json:"makerFeeAssetData"`
	MakerAssetAmount      *big.Int       `json:"makerAssetAmount"`
	MakerFee              *big.Int       `json:"makerFee"`
	TakerAddress          common.Address `json:"takerAddress"`
	TakerAssetData        []byte         `json:"takerAssetData"`
	TakerFeeAssetData     []byte         `json:"takerFeeAssetData"`
	TakerAssetAmount      *big.Int       `json:"takerAssetAmount"`
	TakerFee              *big.Int       `json:"takerFee"`
	SenderAddress         common.Address `json:"senderAddress"`
	DomainHash            common.Hash    `json:"domainHash"`
	FeeRecipientAddress   common.Address `json:"feeRecipientAddress"`
	ExpirationTimeSeconds *big.Int       `json:"expirationTimeSeconds"`
	Salt                  *big.Int       `json:"salt"`

	// Cache hash for performance
	hash *common.Hash
}

// SignedOrder represents a signed 0x order
type SignedOrder struct {
	Order
	Signature []byte `json:"signature"`
}

// SignatureType represents the type of 0x signature encountered
type SignatureType uint8

// SignatureType values
const (
	IllegalSignature SignatureType = iota
	InvalidSignature
	EIP712Signature
	EthSignSignature
	WalletSignature
	ValidatorSignature
	PreSignedSignature
	NSignatureTypesSignature
)

// OrderStatus represents the status of an order as returned from the 0x smart contracts
// as part of OrderInfo
type OrderStatus uint8

// OrderStatus values
const (
	OSInvalid OrderStatus = iota
	OSInvalidMakerAssetAmount
	OSInvalidTakerAssetAmount
	OSFillable
	OSExpired
	OSFullyFilled
	OSCancelled
	OSSignatureInvalid
	OSInvalidMakerAssetData
	OSInvalidTakerAssetData
)

// ContractEventParameters is the parameters of a ContractEvent
type ContractEventParameters interface {
	json.Marshaler
}

// ContractEvent is an event emitted by a smart contract
type ContractEvent struct {
	BlockHash  common.Hash
	TxHash     common.Hash
	TxIndex    uint
	LogIndex   uint
	IsRemoved  bool
	Address    common.Address
	Kind       string
	Parameters ContractEventParameters
}

// MarshalJSON implements a custom JSON marshaller for the ContractEvent type
func (c ContractEvent) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"blockHash":  c.BlockHash.Hex(),
		"txHash":     c.TxHash.Hex(),
		"txIndex":    c.TxIndex,
		"logIndex":   c.LogIndex,
		"isRemoved":  c.IsRemoved,
		"address":    c.Address,
		"kind":       c.Kind,
		"parameters": c.Parameters,
	}
	return json.Marshal(m)
}

// OrderEvent is the order event emitted by Mesh nodes on the "orders" topic
// when calling JSON-RPC method `mesh_subscribe`
type OrderEvent struct {
	OrderHash                common.Hash        `json:"orderHash"`
	SignedOrder              *SignedOrder       `json:"signedOrder"`
	EndState                 OrderEventEndState `json:"endState"`
	FillableTakerAssetAmount *big.Int           `json:"fillableTakerAssetAmount"`
	// All the contract events that triggered this orders re-evaluation. They did not
	// all necessarily cause the orders state change itself, only it's re-evaluation.
	// Since it's state _did_ change, at least one of them did cause the actual state change.
	ContractEvents []*ContractEvent `json:"contractEvents"`
}

type orderEventJSON struct {
	OrderHash                string           `json:"orderHash"`
	SignedOrder              *SignedOrder     `json:"signedOrder"`
	EndState                 string           `json:"endState"`
	FillableTakerAssetAmount string           `json:"fillableTakerAssetAmount"`
	ContractEvents           []*ContractEvent `json:"contractEvents"`
}

// MarshalJSON implements a custom JSON marshaller for the OrderEvent type
func (o OrderEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"orderHash":                o.OrderHash.Hex(),
		"signedOrder":              o.SignedOrder,
		"endState":                 o.EndState,
		"fillableTakerAssetAmount": o.FillableTakerAssetAmount.String(),
		"contractEvents":           o.ContractEvents,
	})
}

// UnmarshalJSON implements a custom JSON unmarshaller for the OrderEvent type
func (o *OrderEvent) UnmarshalJSON(data []byte) error {
	var orderEventJSON orderEventJSON
	err := json.Unmarshal(data, &orderEventJSON)
	if err != nil {
		return err
	}
	return o.fromOrderEventJSON(orderEventJSON)
}

func (o *OrderEvent) fromOrderEventJSON(orderEventJSON orderEventJSON) error {
	o.OrderHash = common.HexToHash(orderEventJSON.OrderHash)
	o.SignedOrder = orderEventJSON.SignedOrder
	o.EndState = OrderEventEndState(orderEventJSON.EndState)
	var ok bool
	o.FillableTakerAssetAmount, ok = math.ParseBig256(orderEventJSON.FillableTakerAssetAmount)
	if !ok {
		return errors.New("Invalid uint256 number encountered for FillableTakerAssetAmount")
	}
	o.ContractEvents = orderEventJSON.ContractEvents
	return nil
}

// OrderEventEndState enumerates all the possible order event types. An OrderEventEndState describes the
// end state of a 0x order after revalidation
type OrderEventEndState string

// OrderEventEndState values
const (
	ESInvalid          = OrderEventEndState("INVALID")
	ESOrderAdded       = OrderEventEndState("ADDED")
	ESOrderFilled      = OrderEventEndState("FILLED")
	ESOrderFullyFilled = OrderEventEndState("FULLY_FILLED")
	ESOrderCancelled   = OrderEventEndState("CANCELLED")
	ESOrderExpired     = OrderEventEndState("EXPIRED")
	// An order becomes unfunded if the maker transfers the balance / changes their
	// allowance backing an order
	ESOrderBecameUnfunded = OrderEventEndState("UNFUNDED")
	// Fillability for an order can increase if a previously processed fill event
	// gets reverted, or if a maker tops up their balance/allowance backing an order
	ESOrderFillabilityIncreased = OrderEventEndState("FILLABILITY_INCREASED")
)

var eip712OrderTypes = signer.Types{
	"EIP712Domain": {
		{
			Name: "name",
			Type: "string",
		},
		{
			Name: "version",
			Type: "string",
		},
		{
			Name: "chainId",
			Type: "uint256",
		},
		{
			Name: "verifyingContract",
			Type: "address",
		},
	},
	"Order": {
		{
			Name: "makerAddress",
			Type: "address",
		},
		{
			Name: "takerAddress",
			Type: "address",
		},
		{
			Name: "feeRecipientAddress",
			Type: "address",
		},
		{
			Name: "senderAddress",
			Type: "address",
		},
		{
			Name: "makerAssetAmount",
			Type: "uint256",
		},
		{
			Name: "takerAssetAmount",
			Type: "uint256",
		},
		{
			Name: "makerFee",
			Type: "uint256",
		},
		{
			Name: "takerFee",
			Type: "uint256",
		},
		{
			Name: "expirationTimeSeconds",
			Type: "uint256",
		},
		{
			Name: "salt",
			Type: "uint256",
		},
		{
			Name: "makerAssetData",
			Type: "bytes",
		},
		{
			Name: "takerAssetData",
			Type: "bytes",
		},
		{
			Name: "makerFeeAssetData",
			Type: "bytes",
		},
		{
			Name: "takerFeeAssetData",
			Type: "bytes",
		},
	},
}

// ResetHash resets the cached order hash. Usually only required for testing.
func (o *Order) ResetHash() {
	o.hash = nil
}

// ComputeOrderHash computes a 0x order hash
func (o *Order) ComputeOrderHash() (common.Hash, error) {
	if o.hash != nil {
		return *o.hash, nil
	}

	exchangeAddress, err := ethereum.GetExchangeAddressForDomainHash(o.DomainHash)
	if err != nil {
		return *o.hash, err
	}
	var domain = signer.TypedDataDomain{
		Name:              "0x Protocol",
		Version:           "2",
		VerifyingContract: exchangeAddress.Hex(),
	}

	var message = map[string]interface{}{
		"makerAddress":          o.MakerAddress.Hex(),
		"takerAddress":          o.TakerAddress.Hex(),
		"senderAddress":         o.SenderAddress.Hex(),
		"feeRecipientAddress":   o.FeeRecipientAddress.Hex(),
		"makerAssetData":        o.MakerAssetData,
		"makerFeeAssetData":     o.MakerFeeAssetData,
		"takerAssetData":        o.TakerAssetData,
		"takerFeeAssetData":     o.TakerFeeAssetData,
		"salt":                  o.Salt,
		"makerFee":              o.MakerFee,
		"takerFee":              o.TakerFee,
		"makerAssetAmount":      o.MakerAssetAmount,
		"takerAssetAmount":      o.TakerAssetAmount,
		"expirationTimeSeconds": o.ExpirationTimeSeconds,
	}

	var typedData = signer.TypedData{
		Types:       eip712OrderTypes,
		PrimaryType: "Order",
		Domain:      domain,
		Message:     message,
	}

	domainSeparator := o.DomainHash.Bytes()
	typedDataHash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
	if err != nil {
		return common.Hash{}, err
	}
	rawData := []byte(fmt.Sprintf("\x19\x01%s%s", string(domainSeparator), string(typedDataHash)))
	hashBytes := keccak256(rawData)
	hash := common.BytesToHash(hashBytes)
	o.hash = &hash
	return hash, nil
}

// SignOrder signs the 0x order with the supplied Signer
func SignOrder(signer ethereum.Signer, order *Order) (*SignedOrder, error) {
	if order == nil {
		return nil, errors.New("cannot sign nil order")
	}
	orderHash, err := order.ComputeOrderHash()
	if err != nil {
		return nil, err
	}

	ecSignature, err := signer.EthSign(orderHash.Bytes(), order.MakerAddress)
	if err != nil {
		return nil, err
	}

	// Generate 0x EthSign Signature (append the signature type byte)
	signature := make([]byte, 66)
	signature[0] = ecSignature.V
	copy(signature[1:33], ecSignature.R[:])
	copy(signature[33:65], ecSignature.S[:])
	signature[65] = byte(EthSignSignature)
	signedOrder := &SignedOrder{
		Order:     *order,
		Signature: signature,
	}
	return signedOrder, nil
}

// SignTestOrder signs the 0x order with the local test signer
func SignTestOrder(order *Order) (*SignedOrder, error) {
	testSigner := ethereum.NewTestSigner()
	signedOrder, err := SignOrder(testSigner, order)
	if err != nil {
		return nil, err
	}
	return signedOrder, nil
}

// ConvertToOrderWithoutDomain re-formats a SignedOrder into the format expected by the 0x
// smart contracts.
func (s *SignedOrder) ConvertToOrderWithoutDomain() wrappers.OrderWithoutDomain {
	OrderWithoutDomain := wrappers.OrderWithoutDomain{
		MakerAddress:          s.MakerAddress,
		TakerAddress:          s.TakerAddress,
		FeeRecipientAddress:   s.FeeRecipientAddress,
		SenderAddress:         s.SenderAddress,
		MakerAssetAmount:      s.MakerAssetAmount,
		TakerAssetAmount:      s.TakerAssetAmount,
		MakerFee:              s.MakerFee,
		TakerFee:              s.TakerFee,
		ExpirationTimeSeconds: s.ExpirationTimeSeconds,
		Salt:                  s.Salt,
		MakerAssetData:        s.MakerAssetData,
		MakerFeeAssetData:     s.MakerFeeAssetData,
		TakerAssetData:        s.TakerAssetData,
		TakerFeeAssetData:     s.TakerFeeAssetData,
	}
	return OrderWithoutDomain
}

// SignedOrderJSON is an unmodified JSON representation of a SignedOrder
type SignedOrderJSON struct {
	MakerAddress          string `json:"makerAddress"`
	MakerAssetData        string `json:"makerAssetData"`
	MakerFeeAssetData     string `json:"makerFeeAssetData"`
	MakerAssetAmount      string `json:"makerAssetAmount"`
	MakerFee              string `json:"makerFee"`
	TakerAddress          string `json:"takerAddress"`
	TakerAssetData        string `json:"takerAssetData"`
	TakerFeeAssetData     string `json:"takerFeeAssetData"`
	TakerAssetAmount      string `json:"takerAssetAmount"`
	TakerFee              string `json:"takerFee"`
	SenderAddress         string `json:"senderAddress"`
	DomainHash            string `json:"domainHash"`
	FeeRecipientAddress   string `json:"feeRecipientAddress"`
	ExpirationTimeSeconds string `json:"expirationTimeSeconds"`
	Salt                  string `json:"salt"`
	Signature             string `json:"signature"`
}

// MarshalJSON implements a custom JSON marshaller for the SignedOrder type
func (s SignedOrder) MarshalJSON() ([]byte, error) {
	makerAssetData := ""
	if len(s.MakerAssetData) != 0 {
		makerAssetData = fmt.Sprintf("0x%s", common.Bytes2Hex(s.MakerAssetData))
	}
	makerFeeAssetData := ""
	if len(s.MakerAssetData) != 0 {
		makerFeeAssetData = fmt.Sprintf("0x%s", common.Bytes2Hex(s.MakerFeeAssetData))
	}
	takerAssetData := ""
	if len(s.TakerAssetData) != 0 {
		takerAssetData = fmt.Sprintf("0x%s", common.Bytes2Hex(s.TakerAssetData))
	}
	takerFeeAssetData := ""
	if len(s.TakerAssetData) != 0 {
		takerFeeAssetData = fmt.Sprintf("0x%s", common.Bytes2Hex(s.TakerFeeAssetData))
	}
	signature := ""
	if len(s.Signature) != 0 {
		signature = fmt.Sprintf("0x%s", common.Bytes2Hex(s.Signature))
	}

	signedOrderBytes, err := json.Marshal(SignedOrderJSON{
		MakerAddress:          strings.ToLower(s.MakerAddress.Hex()),
		MakerAssetData:        makerAssetData,
		MakerFeeAssetData:     makerFeeAssetData,
		MakerAssetAmount:      s.MakerAssetAmount.String(),
		MakerFee:              s.MakerFee.String(),
		TakerAddress:          strings.ToLower(s.TakerAddress.Hex()),
		TakerAssetData:        takerAssetData,
		TakerFeeAssetData:     takerFeeAssetData,
		TakerAssetAmount:      s.TakerAssetAmount.String(),
		TakerFee:              s.TakerFee.String(),
		SenderAddress:         strings.ToLower(s.SenderAddress.Hex()),
		DomainHash:            strings.ToLower(s.DomainHash.Hex()),
		FeeRecipientAddress:   strings.ToLower(s.FeeRecipientAddress.Hex()),
		ExpirationTimeSeconds: s.ExpirationTimeSeconds.String(),
		Salt:                  s.Salt.String(),
		Signature:             signature,
	})
	return signedOrderBytes, err
}

const addressHexLength = 42

// UnmarshalJSON implements a custom JSON unmarshaller for the SignedOrder type
func (s *SignedOrder) UnmarshalJSON(data []byte) error {
	var signedOrderJSON SignedOrderJSON
	err := json.Unmarshal(data, &signedOrderJSON)
	if err != nil {
		return err
	}
	s.MakerAddress = common.HexToAddress(signedOrderJSON.MakerAddress)
	s.MakerAssetData = common.FromHex(signedOrderJSON.MakerAssetData)
	s.MakerFeeAssetData = common.FromHex(signedOrderJSON.MakerFeeAssetData)
	var ok bool
	if signedOrderJSON.MakerAssetAmount != "" {
		s.MakerAssetAmount, ok = math.ParseBig256(signedOrderJSON.MakerAssetAmount)
		if !ok {
			s.MakerAssetAmount = nil
		}
	}
	if signedOrderJSON.MakerFee != "" {
		s.MakerFee, ok = math.ParseBig256(signedOrderJSON.MakerFee)
		if !ok {
			s.MakerFee = nil
		}
	}
	s.TakerAddress = common.HexToAddress(signedOrderJSON.TakerAddress)
	s.TakerAssetData = common.FromHex(signedOrderJSON.TakerAssetData)
	s.TakerFeeAssetData = common.FromHex(signedOrderJSON.TakerFeeAssetData)
	if signedOrderJSON.TakerAssetAmount != "" {
		s.TakerAssetAmount, ok = math.ParseBig256(signedOrderJSON.TakerAssetAmount)
		if !ok {
			s.TakerAssetAmount = nil
		}
	}
	if signedOrderJSON.TakerFee != "" {
		s.TakerFee, ok = math.ParseBig256(signedOrderJSON.TakerFee)
		if !ok {
			s.TakerFee = nil
		}
	}
	s.SenderAddress = common.HexToAddress(signedOrderJSON.SenderAddress)
	s.DomainHash = common.HexToHash(signedOrderJSON.DomainHash)
	s.FeeRecipientAddress = common.HexToAddress(signedOrderJSON.FeeRecipientAddress)
	if signedOrderJSON.ExpirationTimeSeconds != "" {
		s.ExpirationTimeSeconds, ok = math.ParseBig256(signedOrderJSON.ExpirationTimeSeconds)
		if !ok {
			s.ExpirationTimeSeconds = nil
		}
	}
	if signedOrderJSON.Salt != "" {
		s.Salt, ok = math.ParseBig256(signedOrderJSON.Salt)
		if !ok {
			s.Salt = nil
		}
	}
	s.Signature = common.FromHex(signedOrderJSON.Signature)
	return nil
}

// keccak256 calculates and returns the Keccak256 hash of the input data.
func keccak256(data ...[]byte) []byte {
	d := sha3.NewLegacyKeccak256()
	for _, b := range data {
		_, _ = d.Write(b)
	}
	return d.Sum(nil)
}
