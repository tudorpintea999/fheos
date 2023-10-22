package precompiles

import (
	"crypto/rand"
	"errors"
	"math/big"
	"os"

	tfhe "github.com/fhenixprotocol/go-tfhe"
	"github.com/holiman/uint256"
	"golang.org/x/crypto/nacl/box"
)

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

func (s *DepthSet) Add(v int) {
	s.add(v)
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

type verifiedCiphertext struct {
	verifiedDepths *DepthSet
	ciphertext     *tfhe.Ciphertext
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

func isVerifiedAtCurrentDepth(ct *verifiedCiphertext) bool {
	return ct.verifiedDepths.Has(interpreter.GetEVM().GetDepth())
}

func getVerifiedCiphertextFromEVM(ciphertextHash tfhe.Hash) *verifiedCiphertext {
	ct, ok := verifiedCiphertexts[ciphertextHash]
	if ok && isVerifiedAtCurrentDepth(ct) {
		return ct
	}
	return nil
}

func getVerifiedCiphertext(ciphertextHash tfhe.Hash) *verifiedCiphertext {
	return getVerifiedCiphertextFromEVM(ciphertextHash)
}

func get2VerifiedOperands(input []byte) (lhs *verifiedCiphertext, rhs *verifiedCiphertext, err error) {
	if len(input) != 65 {
		return nil, nil, errors.New("input needs to contain two 256-bit sized values and 1 8-bit value")
	}
	lhs = getVerifiedCiphertext(tfhe.BytesToHash(input[0:32]))
	if lhs == nil {
		return nil, nil, errors.New("unverified ciphertext handle")
	}
	rhs = getVerifiedCiphertext(tfhe.BytesToHash(input[32:64]))
	if rhs == nil {
		return nil, nil, errors.New("unverified ciphertext handle")
	}
	err = nil
	return
}

func importCiphertextToEVMAtDepth(ct *tfhe.Ciphertext, depth int) *verifiedCiphertext {
	existing, ok := verifiedCiphertexts[ct.Hash()]
	if ok {
		existing.verifiedDepths.add(depth)
		return existing
	} else {
		verifiedDepths := newDepthSet()
		verifiedDepths.add(depth)
		new := &verifiedCiphertext{
			verifiedDepths,
			ct,
		}
		verifiedCiphertexts[ct.Hash()] = new
		return new
	}
}

func importCiphertextToEVM(ct *tfhe.Ciphertext) *verifiedCiphertext {
	return importCiphertextToEVMAtDepth(ct, interpreter.GetEVM().GetDepth())
}

func importCiphertext(ct *tfhe.Ciphertext) *verifiedCiphertext {
	return importCiphertextToEVM(ct)
}
