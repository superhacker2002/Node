package cmd

import (
	"context"

	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.denetwork.xyz/dfile/dfile-secondary-node/account"
	blockchainprovider "git.denetwork.xyz/dfile/dfile-secondary-node/blockchain_provider"
	"git.denetwork.xyz/dfile/dfile-secondary-node/cleaner"
	"git.denetwork.xyz/dfile/dfile-secondary-node/config"
	"git.denetwork.xyz/dfile/dfile-secondary-node/errs"
	"git.denetwork.xyz/dfile/dfile-secondary-node/logger"
	nodeFile "git.denetwork.xyz/dfile/dfile-secondary-node/node_file"
	"git.denetwork.xyz/dfile/dfile-secondary-node/paths"
	"git.denetwork.xyz/dfile/dfile-secondary-node/server"
	"git.denetwork.xyz/dfile/dfile-secondary-node/upnp"
	"github.com/spf13/cobra"
)

const accLoginFatalError = "Error while account log in"
const ipUpdateFatalError = "Couldn't update public ip info"

// AccountLoginCmd is executed when "login" flag is passed after "account" flag and is used for logging in to an account.
var accountLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "log in a blockchain accounts",
	Long:  "log in a blockchain accounts",
	Run: func(cmd *cobra.Command, args []string) {
		const location = "accountLoginCmd->"
		etherAccount, password, err := account.ValidateUser()
		if err != nil {
			logger.Log(logger.CreateDetails(location, err))
			log.Fatal(accLoginFatalError)
		}

		pathToConfigDir := filepath.Join(paths.AccsDirPath, etherAccount.Address.String(), paths.ConfDirName)

		var nodeConfig config.NodeConfig

		pathToConfigFile := filepath.Join(pathToConfigDir, paths.ConfFileName)

		stat, err := os.Stat(pathToConfigFile)
		err = errs.CheckStatErr(err)
		if err != nil {
			logger.Log(logger.CreateDetails(location, err))
			log.Fatal(accLoginFatalError)
		}

		if stat == nil {
			nodeConfig, err = config.Create(etherAccount.Address.String(), password)
			if err != nil {
				logger.Log(logger.CreateDetails(location, err))
				log.Fatal("couldn't create config file")
			}
		} else {
			confFile, fileBytes, err := nodeFile.Read(pathToConfigFile)
			if err != nil {
				logger.Log(logger.CreateDetails(location, err))
				log.Fatal("couldn't open config file")
			}
			defer confFile.Close()

			err = json.Unmarshal(fileBytes, &nodeConfig)
			if err != nil {
				logger.Log(logger.CreateDetails(location, err))
				log.Fatal("couldn't read config file")
			}

			blockchainprovider.ChainClientAddr = nodeConfig.ChnClntAddr
			blockchainprovider.NFT = nodeConfig.NFT

			if nodeConfig.StorageLimit <= 0 {
				log.Fatal(accLoginFatalError)
			}

			if upnp.InternetDevice != nil {
				ip, err := upnp.InternetDevice.PublicIP()
				if err != nil {
					logger.Log(logger.CreateDetails(location, err))
				}

				if nodeConfig.IpAddress != ip {

					fmt.Println("Updating public ip info...")

					splitIPAddr := strings.Split(ip, ".")

					ctx, _ := context.WithTimeout(context.Background(), time.Minute)

					err = blockchainprovider.UpdateNodeInfo(ctx, etherAccount.Address, password, nodeConfig.HTTPPort, splitIPAddr)
					if err != nil {
						logger.Log(logger.CreateDetails(location, err))
						log.Fatal(ipUpdateFatalError)
					}

					nodeConfig.IpAddress = ip

					err = config.Save(confFile, nodeConfig)
					if err != nil {
						logger.Log(logger.CreateDetails(location, err))
						log.Fatal(ipUpdateFatalError)
					}
				}
			}

			logger.SendLogs = nodeConfig.AgreeSendLogs
		}

		account.IpAddr = fmt.Sprint(nodeConfig.IpAddress, ":", nodeConfig.HTTPPort)

		fmt.Println("Logged in")

		go blockchainprovider.StartMakingProofs(password)

		go cleaner.Start()

		server.Start(nodeConfig.HTTPPort)
	},
}

func init() {
	accountCmd.AddCommand(accountLoginCmd)
}
