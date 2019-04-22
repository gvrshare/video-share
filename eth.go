package seed

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/sirupsen/logrus"
	"github.com/yinhevr/seed/model"
)

//ETH ...
type ETH struct {
	key             string
	ContractAddress string
	DialAddress     string
}

// NewETH ...
func NewETH(key string) *ETH {
	return &ETH{
		key: key,
	}
}

// InfoInput ...
func (eth *ETH) InfoInput(video *model.Video) (e error) {
	return infoInput(eth, video)
}

// ConnectToken ...
func ConnectToken() *BangumiData {
	// Create an IPC based RPC connection to a remote node and instantiate a contract binding
	conn, err := ethclient.Dial("https://ropsten.infura.io/QVsqBu3yopMu2svcHqRj")
	if err != nil {
		logrus.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
	defer conn.Close()

	token, err := NewBangumiData(common.HexToAddress("0xb5eb6bf5eab725e9285d0d27201603ecf31a1d37"), conn)
	if err != nil {
		logrus.Fatalf("Failed to instantiate a Token contract: %v", err)
	}
	logrus.Info(token)

	return token
}

// InfoInput ...
func infoInput(eth *ETH, video *model.Video, index int) (e error) {
	// Create an IPC based RPC connection to a remote node and instantiate a contract binding
	conn, err := ethclient.Dial("https://ropsten.infura.io/QVsqBu3yopMu2svcHqRj")
	if err != nil {
		//logrus.Fatalf("Failed to connect to the Ethereum client: %v", err)
		return err

	}
	defer conn.Close()

	token, err := NewBangumiData(common.HexToAddress("0xb5eb6bf5eab725e9285d0d27201603ecf31a1d37"), conn)
	if err != nil {
		return err
		//logrus.Fatalf("Failed to instantiate a Token contract: %v", err)
	}
	logrus.Info(token)

	//bytes := "key"
	privateKey, err := crypto.HexToECDSA(eth.key)
	if err != nil {
		logrus.Fatal(err)
	}

	opt := bind.NewKeyedTransactor(privateKey)
	logrus.Info(opt)
	transaction, err := token.InfoInput(opt,
		video.Bangumi,
		video.Poster,
		video.Role[0],
		video.VideoGroupList[0].Object[0].Link.Hash,
		video.Alias[0],
		video.VideoGroupList[0].Sharpness,
		video.VideoGroupList[0].Episode,
		video.VideoGroupList[0].TotalEpisode,
		video.VideoGroupList[0].Season,
		video.VideoGroupList[0].Output,
		"",
		"")
	if err != nil {
		return err
	}
	ctx := context.Background()
	receipt, err := bind.WaitMined(ctx, conn, transaction)
	if err != nil {
		//logrus.Fatalf("tx mining error:%v\n", err)
		return err
	}
	//fmt.Printf("tx is :%+v\n", transaction)
	fmt.Printf("receipt is :%x\n", string(receipt.TxHash[:]))
	return nil
}
