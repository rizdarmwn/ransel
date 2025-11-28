package main

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func transferETH(client *ethclient.Client, from *ecdsa.PrivateKey, to common.Address, amount *big.Int) error {
	ctx := context.Background()
	publicKey := from.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("invalid public key type")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	chainid, err := client.ChainID(ctx)
	if err != nil {
		return err
	}

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return err
	}

	tipCap, _ := client.SuggestGasTipCap(ctx)
	feeCap, _ := client.SuggestGasPrice(ctx)

	tx := types.NewTx(
		&types.DynamicFeeTx{
			ChainID:   chainid,
			Nonce:     nonce,
			GasTipCap: tipCap,
			GasFeeCap: feeCap,
			Gas:       21000,
			To:        &to,
			Value:     amount,
			Data:      nil,
		},
	)

	signedTx, err := types.SignTx(tx, types.NewLondonSigner(chainid), from)

	return client.SendTransaction(ctx, signedTx)
}
