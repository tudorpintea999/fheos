// SPDX-License-Identifier: BSD-3-Clause-Clear

pragma solidity >=0.4.21 <0.9.0;

interface FheOps {
    function add(
        bytes memory input,
        uint32 inputLen
    ) external view returns (bytes memory);

    function reencrypt(bytes memory input, uint32 inputLen) external view returns (bytes memory);
    function trivialEncrypt(bytes memory input) external view returns (bytes memory);
}
