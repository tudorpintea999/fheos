package precompiles

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/core/vm"
	tfhe "github.com/fhenixprotocol/go-tfhe"
	"github.com/holiman/uint256"
	"golang.org/x/crypto/nacl/box"
)

type tomlConfigOptions struct {
	Oracle struct {
		Mode              string
		OracleDBAddress   string
		RequireRetryCount uint8
	}

	Tfhe struct {
		CiphertextsToGarbageCollect           uint64
		CiphertextsGarbageCollectIntervalSecs uint64
	}
}

var tomlConfig tomlConfigOptions

type DepthSet struct {
	m map[int]struct{}
}

func newDepthSet() *DepthSet {
	s := &DepthSet{}
	s.m = make(map[int]struct{})
	return s
}

func (s *DepthSet) add(v int) {
	s.m[v] = struct{}{}
}

func (s *DepthSet) del(v int) {
	delete(s.m, v)
}

func (s *DepthSet) has(v int) bool {
	_, found := s.m[v]
	return found
}

func (s *DepthSet) Has(v int) bool {
	return s.has(v)
}

func (s *DepthSet) count() int {
	return len(s.m)
}

func (from *DepthSet) clone() (to *DepthSet) {
	to = newDepthSet()
	for k := range from.m {
		to.add(k)
	}
	return
}

func toEVMBytes(input []byte) []byte {
	len := uint64(len(input))
	lenBytes32 := uint256.NewInt(len).Bytes32()
	ret := make([]byte, 0, len+32)
	ret = append(ret, lenBytes32[:]...)
	ret = append(ret, input...)
	return ret
}

func classicalPublicKeyEncrypt(value *big.Int, userPublicKey []byte) ([]byte, error) {
	encrypted, err := box.SealAnonymous(nil, value.Bytes(), (*[32]byte)(userPublicKey), rand.Reader)
	if err != nil {
		return nil, err
	}
	return encrypted, nil
}

func encryptToUserKey(value *big.Int, pubKey []byte) ([]byte, error) {
	ct, err := classicalPublicKeyEncrypt(value, pubKey)
	if err != nil {
		return nil, err
	}

	// TODO: for testing
	err = os.WriteFile("/tmp/public_encrypt_result", ct, 0644)
	if err != nil {
		return nil, err
	}

	return ct, nil
}

func getCiphertext(ciphertextHash tfhe.Hash) *tfhe.Ciphertext {
	ct, ok := ctHashMap[ciphertextHash]
	if ok {
		return ct
	}
	return nil
}

func get2VerifiedOperands(input []byte) (lhs *tfhe.Ciphertext, rhs *tfhe.Ciphertext, err error) {
	if len(input) != 65 {
		return nil, nil, errors.New("input needs to contain two 256-bit sized values and 1 8-bit value")
	}
	lhs = getCiphertext(tfhe.BytesToHash(input[0:32]))
	if lhs == nil {
		return nil, nil, errors.New("unverified ciphertext handle")
	}
	rhs = getCiphertext(tfhe.BytesToHash(input[32:64]))
	if rhs == nil {
		return nil, nil, errors.New("unverified ciphertext handle")
	}
	err = nil
	return
}

func importCiphertext(ct *tfhe.Ciphertext) *tfhe.Ciphertext {
	existing, ok := ctHashMap[ct.Hash()]
	if ok {
		return existing
	} else {
		ctHashMap[ct.Hash()] = ct
		return ct
	}
}

func importRandomCiphertext(t tfhe.UintType) []byte {
	//ct := new(tfhe.Ciphertext)
	//ct.MakeRandom(t)
	ct, err := tfhe.NewRandomCipherText(t)
	if err != nil {
		panic(fmt.Sprintf("Failed to create random ciphertext of size: %d", t))
	}

	importCiphertext(ct)
	ctHash := ct.Hash()
	return ctHash[:]
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

// Puts the given ciphertext as a require to the oracle DB or exits the process on errors.
// Returns the require value.
func putRequire(ct *tfhe.Ciphertext, interpreter *vm.EVMInterpreter) bool {
	logger := interpreter.GetEVM().Logger

	plaintext, err := tfhe.Decrypt(*ct)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to decrypt value: %s", err))
		panic(err)
	}

	result, err := tfhe.StoreRequire(ct, plaintext)
	if err != nil {
		logger.Error("Failed to store require in DB")
		panic("Failed to decrypt value")
	}

	return result
}

// Gets the given require from the oracle DB and returns its value.
// Exits the process on errors or signature verification failure.
func getRequire(ct *tfhe.Ciphertext, interpreter *vm.EVMInterpreter) bool {
	result, err := tfhe.CheckRequire(ct)
	if err != nil {
		interpreter.GetEVM().Logger.Error("Error verifying require", err)
		return false
	}

	return result
}

func evaluateRequire(ct *tfhe.Ciphertext, interpreter *vm.EVMInterpreter) bool {
	mode := strings.ToLower(tomlConfig.Oracle.Mode)
	switch mode {
	case "oracle":
		return putRequire(ct, interpreter)
	case "node":
		return getRequire(ct, interpreter)
	}
	interpreter.GetEVM().Logger.Error("evaluateRequire invalid mode", "mode", mode)
	// exitProcess()
	return false
}
