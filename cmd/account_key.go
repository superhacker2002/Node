package cmd

import (
	"encoding/hex"
	"fmt"
	"log"

	"git.denetwork.xyz/dfile/dfile-secondary-node/account"
	"git.denetwork.xyz/dfile/dfile-secondary-node/logger"
	"git.denetwork.xyz/dfile/dfile-secondary-node/paths"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/spf13/cobra"
)

const showKeyFatalMessage = "Fatal error while extracting private key"

var showKeyCmd = &cobra.Command{
	Use:   "key",
	Short: "discloses your private key",
	Long:  "discloses your private key",
	Run: func(cmd *cobra.Command, args []string) {
		const actLoc = "showKeyCmd->"
		fmt.Println("Never disclose this key. Anyone with your private keys can steal any assets held in your account")

		etherAccount, password, err := account.ValidateUser()
		if err != nil {
			logger.Log(logger.CreateDetails(actLoc, err))
			log.Fatal(showKeyFatalMessage)
		}

		ks := keystore.NewKeyStore(paths.AccsDirPath, keystore.StandardScryptN, keystore.StandardScryptP)

		keyJson, err := ks.Export(*etherAccount, password, password)
		if err != nil {
			logger.Log(logger.CreateDetails(actLoc, err))
			log.Fatal(showKeyFatalMessage)
		}

		key, err := keystore.DecryptKey(keyJson, password)
		if err != nil {
			logger.Log(logger.CreateDetails(actLoc, err))
			log.Fatal(showKeyFatalMessage)
		}

		fmt.Println("Private Key:", hex.EncodeToString(key.PrivateKey.D.Bytes()))
	},
}

func init() {
	accountCmd.AddCommand(showKeyCmd)
}
