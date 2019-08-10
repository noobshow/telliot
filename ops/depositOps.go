package ops

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	tellorCommon "github.com/tellor-io/TellorMiner/common"
	"github.com/tellor-io/TellorMiner/config"
	tellor "github.com/tellor-io/TellorMiner/contracts"
	tellor1 "github.com/tellor-io/TellorMiner/contracts1"
	"github.com/tellor-io/TellorMiner/rpc"
	"github.com/tellor-io/TellorMiner/util"
)

/**
 * This is the operational deposit component. Its purpose is to deposit Tellor Tokens so you can mine
 */

var (
	depositLog = util.NewLogger("ops", "DepositOp")
)

func Deposit(ctx context.Context) error {
	cfg, err := config.GetConfig()
	if err != nil {
		depositLog.Error("Problem getting config: %+v", err)
		return err
	}
	client := ctx.Value(tellorCommon.ClientContextKey).(rpc.ETHClient)

	privateKey, err := crypto.HexToECDSA(cfg.PrivateKey)
	if err != nil {
		depositLog.Error("Problem getting private key: %+v", err)
		return err
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		depositLog.Error("Problem extract public key")
		return fmt.Errorf("error casting public key to ECDSA")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		depositLog.Error("Problem getting pending nonce: %+v", err)
		return err
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		depositLog.Error("Problem getting gas price: %+v", err)
		return err
	}

	balance, err := client.BalanceAt(context.Background(), fromAddress, nil)
	if err != nil {
		depositLog.Error("Problem getting balance: %+v", err)
		return err
	}

	cost := new(big.Int)
	cost.Mul(gasPrice, big.NewInt(700000))
	if balance.Cmp(cost) < 0 {
		//FIXME: notify someone that we're out of funds!
		depositLog.Error("Insufficient funds")
		return fmt.Errorf("Insufficient funds to send transaction: %v < %v", balance, cost)
	}

	auth := bind.NewKeyedTransactor(privateKey)
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)      // in wei
	auth.GasLimit = uint64(3000000) // in units
	auth.GasPrice = gasPrice

	contractAddress := common.HexToAddress(cfg.ContractAddress)

	instance, err := tellor.NewTellorMaster(contractAddress, client)
	if err != nil {
		depositLog.Error("Problem creating contract: %+v", err)
		//log.Fatal(err)
		return err
	}

	balance, err = instance.BalanceOf(nil, fromAddress)
	depositLog.Info("Balance: %v\n", balance)
	if err != nil {
		depositLog.Error("Problem getting balance: %+v", err)
		return err
	}
	amt := big.NewInt(1000)
	if balance.Cmp(amt) < 0 {
		depositLog.Error("Insufficient token balance: %+v", balance)
		fmt.Println("You must have the amount you want to send")
		return nil
	}

	instance2 := ctx.Value(tellorCommon.TransactorContractContextKey).(*tellor1.TellorTransactor)

	tx, err := instance2.DepositStake(auth)
	if err != nil {
		depositLog.Error("Could not deposit stake: %+v", err)
		return err
	}

	depositLog.Info("tx sent: %s", tx.Hash().Hex())

	return nil
}
