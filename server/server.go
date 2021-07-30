package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"math/big"
	"mime/multipart"
	"os/signal"
	"strings"

	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	blockchainprovider "git.denetwork.xyz/dfile/dfile-secondary-node/blockchain_provider"
	"git.denetwork.xyz/dfile/dfile-secondary-node/config"
	"git.denetwork.xyz/dfile/dfile-secondary-node/logger"
	nodeAbi "git.denetwork.xyz/dfile/dfile-secondary-node/node_abi"
	"git.denetwork.xyz/dfile/dfile-secondary-node/paths"
	"git.denetwork.xyz/dfile/dfile-secondary-node/shared"
	"git.denetwork.xyz/dfile/dfile-secondary-node/upnp"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

type updatedFsInfo struct {
	NewFs            []string
	Nonce            string
	SignedFsRootHash string
}
type NodeAddressResponse struct {
	NodeAddress string `json:"node_address"`
}

const gbBytes = int64(1024 * 1024 * 1024)
const oneHunderdMBBytes = int64(1024 * 1024 * 100)
const serverStartFatalMessage = "Couldn't start server"

func Start(port string) {
	const logLoc = "server.Start->"
	r := mux.NewRouter()

	r.HandleFunc("/upload/{size}", SaveFiles).Methods("POST")
	r.HandleFunc("/download/{spAddress}/{fileKey}/{signature}", ServeFiles).Methods("GET")
	r.HandleFunc("/update_fs/{spAddress}/{signedFsys}", updateFsInfo).Methods("POST")
	r.HandleFunc("/copy/{size}", CopyFile).Methods("POST")
	r.HandleFunc("/backup/{size}", BackUp).Methods("POST")

	corsOpts := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodOptions,
		},

		AllowedHeaders: []string{
			"Accept",
			"Content-Type",
			"Content-Length",
			"Accept-Encoding",
			"X-CSRF-Token",
			"Authorization",
		},
	})

	intPort, err := strconv.Atoi(port)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		log.Fatal(serverStartFatalMessage)
	}

	if upnp.InternetDevice != nil {
		upnp.InternetDevice.Forward(intPort, "node")
		defer upnp.InternetDevice.Close(intPort)
	}

	fmt.Println("Dfile node is ready and started listening on port: " + port)

	server := http.Server{
		Addr:    ":" + port,
		Handler: corsOpts.Handler(checkSignature(r)),
	}

	go func() {
		err = server.ListenAndServe()
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			log.Fatal(serverStartFatalMessage)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	<-stop

	err = server.Shutdown(context.Background())
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		log.Fatal(err)
	}
}

// ====================================================================================

func checkSignature(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// splittedPath := strings.Split(r.URL.Path, "/")
		// signature := splittedPath[len(splittedPath)-1]
		// splittedPath = splittedPath[:len(splittedPath)-1]
		// reqURL := strings.Join(splittedPath, "/")

		// verified, err := verifySignature(sessionKeyBytes, reqURL, signature)
		// if err != nil {
		// 	http.Error(w, "session key verification error", 500)
		// 	return
		// }

		// if !verified {
		// 	http.Error(w, "wrong session key", http.StatusForbidden)
		// }

		h.ServeHTTP(w, r)
	})
}

// ========================================================================================================

func SaveFiles(w http.ResponseWriter, req *http.Request) {
	const logLoc = "server.SaveFiles->"

	pathToConfig := filepath.Join(paths.AccsDirPath, shared.NodeAddr.String(), paths.ConfDirName, paths.ConfFileName)

	intFileSize, _, _, err := checkSpace(req, pathToConfig)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, "space check problem", 500)
		return
	}

	spData, err := parseRequest(req)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "Parse multiform problem", 400)
		return
	}

	addressPath := filepath.Join(paths.AccsDirPath, shared.NodeAddr.String(), paths.StorageDirName, spData.Address)

	stat, err := os.Stat(addressPath)
	err = shared.CheckStatErr(err)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "File saving problem", 500)
		return
	}

	if stat == nil {
		err = os.Mkdir(addressPath, 0700)
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, "File saving problem", 500)
			return
		}
	}

	shared.MU.Lock()
	spFsFile, err := os.Create(filepath.Join(addressPath, paths.SpFsFilename))
	if err != nil {
		shared.MU.Unlock()
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "File saving problem", 500)
		return
	}
	defer spFsFile.Close()

	js, err := json.Marshal(spData)
	if err != nil {
		shared.MU.Unlock()
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "File saving problem", 500)
		return
	}

	_, err = spFsFile.Write(js)
	if err != nil {
		shared.MU.Unlock()
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "File saving problem", 500)
		return
	}

	spFsFile.Sync()
	spFsFile.Close()
	shared.MU.Unlock()

	reqFileParts := req.MultipartForm.File["files"]

	const eightKB = 8192

	oneMBHashes := make([]string, 0, len(reqFileParts))

	for _, reqFilePart := range reqFileParts {

		eightKBHashes := make([]string, 0, 128)

		var buf bytes.Buffer

		rqFile, err := reqFilePart.Open()
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, "File check problem", 500)
			return
		}

		_, err = io.Copy(&buf, rqFile)
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			restoreMemoryInfo(pathToConfig, intFileSize)
			rqFile.Close()
			http.Error(w, "File check problem", 500)
			return
		}

		rqFile.Close()

		bufBytes := buf.Bytes()

		for i := 0; i < len(bufBytes); i += eightKB {
			hSum := sha256.Sum256(bufBytes[i : i+eightKB])
			eightKBHashes = append(eightKBHashes, hex.EncodeToString(hSum[:]))
		}

		oneMBHash, _, err := shared.CalcRootHash(eightKBHashes)
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, "Wrong file", 400)
			return
		}

		if reqFilePart.Filename != oneMBHash {
			err := errors.New("wrong file")
			logger.Log(logger.CreateDetails(logLoc, err))
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, err.Error(), 400)
			return
		}

		oneMBHashes = append(oneMBHashes, oneMBHash)

	}

	fsContainsFile := false

	var wholeFileHash string

	if len(oneMBHashes) == 1 {
		wholeFileHash = oneMBHashes[0]
	} else {
		sort.Strings(oneMBHashes)
		wholeFileHash, _, err = shared.CalcRootHash(oneMBHashes)
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, "Wrong file", 400)
			return
		}
	}

	for _, fileHash := range spData.Fs {
		if fileHash == wholeFileHash {
			fsContainsFile = true
		}
	}

	if !fsContainsFile {
		err := errors.New("wrong file")
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, err.Error(), 400)
		return
	}

	count := 1
	total := len(oneMBHashes)

	for _, reqFilePart := range reqFileParts {
		rqFile, err := reqFilePart.Open()
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			deleteFileParts(addressPath, oneMBHashes)
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, "File saving problem", 500)
			return
		}
		defer rqFile.Close()

		pathToFile := filepath.Join(addressPath, reqFilePart.Filename)

		newFile, err := os.Create(pathToFile)
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			deleteFileParts(addressPath, oneMBHashes)
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, "File saving problem", 500)
			return
		}
		defer newFile.Close()

		_, err = io.Copy(newFile, rqFile)
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			deleteFileParts(addressPath, oneMBHashes)
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, "File saving problem", 500)
			return
		}

		logger.Log("Saved file " + reqFilePart.Filename + " (" + fmt.Sprint(count) + "/" + fmt.Sprint(total) + ")" + " from " + spData.Address) //TODO remove

		newFile.Sync()
		rqFile.Close()
		newFile.Close()

		count++
	}

	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "OK")
}

// ====================================================================================

func deleteFileParts(addressPath string, fileHashes []string) {

	logger.Log("deleting file parts after error...")

	for _, hash := range fileHashes {
		pathToFile := filepath.Join(addressPath, hash)

		os.Remove(pathToFile)
	}
}

// ====================================================================================

func ServeFiles(w http.ResponseWriter, req *http.Request) {
	const logLoc = "server.ServeFiles->"

	vars := mux.Vars(req)
	spAddress := vars["spAddress"]
	fileKey := vars["fileKey"]
	signatureFromReq := vars["signature"]

	signature, err := hex.DecodeString(signatureFromReq)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, "File serving problem", 400)
		return
	}

	hash := sha256.Sum256([]byte(fileKey + spAddress))

	sigPublicKey, err := crypto.SigToPub(hash[:], signature)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, "File serving problem", 400)
		return
	}

	signatureAddress := crypto.PubkeyToAddress(*sigPublicKey)

	if spAddress != signatureAddress.String() {
		logger.Log(logger.CreateDetails(logLoc, errors.New("wrong signature")))
		http.Error(w, "Wrong signature", http.StatusForbidden)
		return
	}

	pathToFile := filepath.Join(paths.AccsDirPath, shared.NodeAddr.String(), paths.StorageDirName, spAddress, fileKey)

	_, err = os.Stat(pathToFile)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	logger.Log("serving file: " + fileKey)

	http.ServeFile(w, req, pathToFile)
}

// ====================================================================================

func updateFsInfo(w http.ResponseWriter, req *http.Request) {
	const logLoc = "server.UpdateFsInfo->"

	const httpErrorMsg = "Fs info update problem"

	vars := mux.Vars(req)
	spAddress := vars["spAddress"]
	signedFsys := vars["signedFsys"]

	addressPath := filepath.Join(paths.AccsDirPath, shared.NodeAddr.String(), paths.StorageDirName, spAddress)

	_, err := os.Stat(addressPath)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, errors.New("no files of "+spAddress)))
		return
	}

	shared.MU.Lock()
	spFsFile, fileBytes, err := shared.ReadFile(filepath.Join(addressPath, paths.SpFsFilename))
	if err != nil {
		shared.MU.Unlock()
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, httpErrorMsg, 500)
		return
	}
	defer spFsFile.Close()

	var spFs shared.StorageProviderData

	err = json.Unmarshal(fileBytes, &spFs)
	if err != nil {
		shared.MU.Unlock()
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, httpErrorMsg, 500)
		return
	}

	spFsFile.Close()
	shared.MU.Unlock()

	body, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, httpErrorMsg, 500)
		return
	}
	defer req.Body.Close()

	var updatedFs updatedFsInfo

	err = json.Unmarshal(body, &updatedFs)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, httpErrorMsg, 500)
		return
	}

	newNonceInt, err := strconv.Atoi(updatedFs.Nonce)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, httpErrorMsg, 400)
		return
	}

	currentNonceInt, err := strconv.Atoi(spFs.Nonce)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, httpErrorMsg, 400)
		return
	}

	if newNonceInt < currentNonceInt {
		logger.Log(spAddress + " fs info is up to date")
		http.Error(w, httpErrorMsg, 400)
		return
	}

	nonceHex := strconv.FormatInt(int64(newNonceInt), 16)

	nonceBytes, err := hex.DecodeString(nonceHex)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, httpErrorMsg, 400)
		return
	}

	nonce32 := make([]byte, 32-len(nonceBytes))
	nonce32 = append(nonce32, nonceBytes...)

	sort.Strings(updatedFs.NewFs)

	concatFsHashes := ""

	for _, hash := range updatedFs.NewFs {
		concatFsHashes += hash
	}

	fsTreeNonceBytes := append([]byte(concatFsHashes), nonce32...)

	fsTreeNonceSha := sha256.Sum256(fsTreeNonceBytes)

	fsysSignature, err := hex.DecodeString(signedFsys)
	if err != nil {
		http.Error(w, httpErrorMsg, 500)
		return
	}

	sigPublicKey, err := crypto.SigToPub(fsTreeNonceSha[:], fsysSignature)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, "Wrong signature", 400)
		return
	}

	signatureAddress := crypto.PubkeyToAddress(*sigPublicKey)

	if spAddress != signatureAddress.String() {
		logger.Log(logger.CreateDetails(logLoc, errors.New("wrong signature")))
		http.Error(w, "Wrong signature", http.StatusForbidden)
		return
	}

	fsRootHash, fsTree, err := shared.CalcRootHash(updatedFs.NewFs)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, httpErrorMsg, 500)
		return
	}

	rootSignature, err := hex.DecodeString(updatedFs.SignedFsRootHash)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, httpErrorMsg, 500)
		return
	}

	fsRootBytes, err := hex.DecodeString(fsRootHash)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, httpErrorMsg, 500)
		return
	}

	fsRootNonceBytes := append(fsRootBytes, nonce32...)

	hash := sha256.Sum256(fsRootNonceBytes)

	sigPublicKey, err = crypto.SigToPub(hash[:], rootSignature)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, httpErrorMsg, 500)
		return
	}

	signatureAddress = crypto.PubkeyToAddress(*sigPublicKey)

	if spAddress != signatureAddress.String() {
		logger.Log(logger.CreateDetails(logLoc, errors.New("wrong signature")))
		http.Error(w, "Wrong signature", http.StatusForbidden)
		return
	}

	shared.MU.Lock()

	spFsFile, err = os.Create(filepath.Join(addressPath, paths.SpFsFilename))
	if err != nil {
		shared.MU.Unlock()
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, httpErrorMsg, 500)
		return
	}
	defer spFsFile.Close()

	spFs = shared.StorageProviderData{
		Nonce:        updatedFs.Nonce,
		SignedFsRoot: updatedFs.SignedFsRootHash,
		Tree:         fsTree,
	}

	js, err := json.Marshal(spFs)
	if err != nil {
		shared.MU.Unlock()
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, httpErrorMsg, 500)
		return
	}

	_, err = spFsFile.Write(js)
	if err != nil {
		shared.MU.Unlock()
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, httpErrorMsg, 500)
		return
	}

	spFsFile.Sync()
	shared.MU.Unlock()

}

func getNodeIP(nodeInfo nodeAbi.SimpleMetaDataDeNetNode) string {
	ipBuilder := strings.Builder{}
	for i, v := range nodeInfo.IpAddress {
		stringPart := strconv.Itoa(int(v))
		ipBuilder.WriteString(stringPart)

		if i < 3 {
			ipBuilder.WriteString(".")
		}
	}

	stringPort := strconv.Itoa(int(nodeInfo.Port))
	ipBuilder.WriteString(":")
	ipBuilder.WriteString(stringPort)

	return ipBuilder.String()
}

// ====================================================================================

func CopyFile(w http.ResponseWriter, req *http.Request) {
	logLoc := "server.CopyFile->"
	fmt.Println("Copy file")

	pathToConfig := filepath.Join(paths.AccsDirPath, shared.NodeAddr.String(), paths.ConfDirName, paths.ConfFileName)

	intFileSize, enoughSpace, nodeConfig, err := checkSpace(req, pathToConfig)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, "space check problem", 500)
		return
	}

	spData, err := parseRequest(req)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "Parse multiform problem", 400)
		return
	}

	addressPath := filepath.Join(paths.AccsDirPath, shared.NodeAddr.String(), paths.StorageDirName, spData.Address)

	stat, err := os.Stat(addressPath)
	err = shared.CheckStatErr(err)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "File saving problem", 500)
		return
	}

	if stat == nil {
		err = os.Mkdir(addressPath, 0700)
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, "File saving problem", 500)
			return
		}
	}

	if !enoughSpace {
		nftNode, err := blockchainprovider.GetNodeNFT()
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			http.Error(w, err.Error(), 400)
			return
		}

		total, err := nftNode.TotalSupply(&bind.CallOpts{})
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			http.Error(w, err.Error(), 400)
			return
		}

		intTotal := total.Int64()

		fastReq := fasthttp.AcquireRequest()
		fastResp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseRequest(fastReq)
		defer fasthttp.ReleaseResponse(fastResp)

		for i := int64(0); i < intTotal; i++ {
			node, err := nftNode.GetNodeById(&bind.CallOpts{}, big.NewInt(i))
			if err != nil {
				fmt.Println(err)
				continue
			}

			nodeIP := getNodeIP(node)

			if nodeIP == nodeConfig.IpAddress+":"+nodeConfig.HTTPPort {
				continue
			}

			url := "http://" + nodeIP
			fastReq.Reset()
			fastResp.Reset()

			fastReq.Header.SetRequestURI(url)
			fastReq.Header.SetMethod("GET")
			fastReq.Header.Set("Connection", "close")

			err = fasthttp.Do(fastReq, fastResp)
			if err != nil {
				continue
			}

			nodeAddress, err := backUpTo(nodeIP, addressPath, req.MultipartForm, intFileSize)
			if err != nil {
				continue
			}

			resp := NodeAddressResponse{
				NodeAddress: nodeAddress,
			}

			js, err := json.Marshal(resp)
			if err != nil {
				continue
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(js)
			return
		}

		http.Error(w, "no available nodes", 500)
		return
	}

	shared.MU.Lock()
	spFsFile, err := os.Create(filepath.Join(addressPath, paths.SpFsFilename))
	if err != nil {
		shared.MU.Unlock()
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "File saving problem", 500)
		return
	}
	defer spFsFile.Close()

	js, err := json.Marshal(spData)
	if err != nil {
		shared.MU.Unlock()
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "File saving problem", 500)
		return
	}

	_, err = spFsFile.Write(js)
	if err != nil {
		shared.MU.Unlock()
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "File saving problem", 500)
		return
	}

	spFsFile.Sync()
	spFsFile.Close()
	shared.MU.Unlock()

	hashes := req.MultipartForm.File["hashes"]
	hashesFile, err := hashes[0].Open()
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "File saving problem", 500)
		return
	}

	hashesBody, err := io.ReadAll(hashesFile)
	if err != nil {
		hashesFile.Close()
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "File saving problem", 500)
		return
	}

	hashDif := make(map[string]string)
	err = json.Unmarshal(hashesBody, &hashDif)
	if err != nil {
		hashesFile.Close()
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "File saving problem", 500)
		return
	}

	hashesFile.Close()

	for old, new := range hashDif {
		path := filepath.Join(addressPath, old)
		file, err := os.Open(path)
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, "File saving problem", 500)
			return
		}

		defer file.Close()

		newPath := filepath.Join(addressPath, new)
		newFile, err := os.Create(newPath)
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, "File saving problem", 500)
			return
		}

		defer newFile.Close()

		_, err = io.Copy(newFile, file)
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, "File saving problem", 500)
			return
		}

		newFile.Sync()
		newFile.Close()
	}

	resp := NodeAddressResponse{
		NodeAddress: nodeConfig.IpAddress + ":" + nodeConfig.HTTPPort,
	}

	js, err = json.Marshal(resp)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

// ====================================================================================

func BackUp(w http.ResponseWriter, req *http.Request) {
	logLoc := "server.BackUp->"

	pathToConfig := filepath.Join(paths.AccsDirPath, shared.NodeAddr.String(), paths.ConfDirName, paths.ConfFileName)

	intFileSize, _, _, err := checkSpace(req, pathToConfig)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		http.Error(w, "space check problem", 500)
		return
	}

	spData, err := parseRequest(req)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "Parse multiform problem", 400)
		return
	}

	addressPath := filepath.Join(paths.AccsDirPath, shared.NodeAddr.String(), paths.StorageDirName, spData.Address)

	stat, err := os.Stat(addressPath)
	err = shared.CheckStatErr(err)
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "File saving problem", 500)
		return
	}

	if stat == nil {
		err = os.Mkdir(addressPath, 0700)
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, "File saving problem", 500)
			return
		}
	}

	shared.MU.Lock()
	spFsFile, err := os.Create(filepath.Join(addressPath, paths.SpFsFilename))
	if err != nil {
		shared.MU.Unlock()
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "File saving problem", 500)
		return
	}
	defer spFsFile.Close()

	js, err := json.Marshal(spData)
	if err != nil {
		shared.MU.Unlock()
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "File saving problem", 500)
		return
	}

	_, err = spFsFile.Write(js)
	if err != nil {
		shared.MU.Unlock()
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "File saving problem", 500)
		return
	}

	spFsFile.Sync()
	spFsFile.Close()
	shared.MU.Unlock()

	reqFileParts := req.MultipartForm.File["files"]

	const eightKB = 8192

	oneMBHashes := make([]string, 0, len(reqFileParts))

	for _, reqFilePart := range reqFileParts {

		eightKBHashes := make([]string, 0, 128)

		var buf bytes.Buffer

		rqFile, err := reqFilePart.Open()
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, "File check problem", 500)
			return
		}

		_, err = io.Copy(&buf, rqFile)
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			restoreMemoryInfo(pathToConfig, intFileSize)
			rqFile.Close()
			http.Error(w, "File check problem", 500)
			return
		}

		rqFile.Close()

		bufBytes := buf.Bytes()

		for i := 0; i < len(bufBytes); i += eightKB {
			hSum := sha256.Sum256(bufBytes[i : i+eightKB])
			eightKBHashes = append(eightKBHashes, hex.EncodeToString(hSum[:]))
		}

		oneMBHash, _, err := shared.CalcRootHash(eightKBHashes)
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, "Wrong file", 400)
			return
		}

		if reqFilePart.Filename != oneMBHash {
			err := errors.New("wrong file")
			logger.Log(logger.CreateDetails(logLoc, err))
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, err.Error(), 400)
			return
		}

		oneMBHashes = append(oneMBHashes, oneMBHash)

	}

	fsContainsFile := false

	var wholeFileHash string

	if len(oneMBHashes) == 1 {
		wholeFileHash = oneMBHashes[0]
	} else {
		sort.Strings(oneMBHashes)
		wholeFileHash, _, err = shared.CalcRootHash(oneMBHashes)
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, "Wrong file", 400)
			return
		}
	}

	for _, fileHash := range spData.Fs {
		if fileHash == wholeFileHash {
			fsContainsFile = true
		}
	}

	if !fsContainsFile {
		err := errors.New("wrong file")
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, err.Error(), 400)
		return
	}

	count := 1
	total := len(oneMBHashes)

	hashes := req.MultipartForm.File["hashes"]
	hashesFile, err := hashes[0].Open()
	if err != nil {
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "File saving problem", 500)
		return
	}

	hashesBody, err := io.ReadAll(hashesFile)
	if err != nil {
		hashesFile.Close()
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "File saving problem", 500)
		return
	}

	hashDif := make(map[string]string)
	err = json.Unmarshal(hashesBody, &hashDif)
	if err != nil {
		hashesFile.Close()
		logger.Log(logger.CreateDetails(logLoc, err))
		restoreMemoryInfo(pathToConfig, intFileSize)
		http.Error(w, "File saving problem", 500)
		return
	}

	hashesFile.Close()

	for _, reqFilePart := range reqFileParts {
		rqFile, err := reqFilePart.Open()
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			deleteFileParts(addressPath, oneMBHashes)
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, "File saving problem", 500)
			return
		}
		defer rqFile.Close()

		pathToFile := filepath.Join(addressPath, hashDif[reqFilePart.Filename])

		newFile, err := os.Create(pathToFile)
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			deleteFileParts(addressPath, oneMBHashes)
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, "File saving problem", 500)
			return
		}
		defer newFile.Close()

		_, err = io.Copy(newFile, rqFile)
		if err != nil {
			logger.Log(logger.CreateDetails(logLoc, err))
			deleteFileParts(addressPath, oneMBHashes)
			restoreMemoryInfo(pathToConfig, intFileSize)
			http.Error(w, "File saving problem", 500)
			return
		}

		logger.Log("Saved file " + hashDif[reqFilePart.Filename] + " (" + fmt.Sprint(count) + "/" + fmt.Sprint(total) + ")" + " from " + spData.Address) //TODO remove

		newFile.Sync()
		rqFile.Close()
		newFile.Close()

		count++
	}

	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "OK")
}

// ====================================================================================

func checkSpace(r *http.Request, pathToConfig string) (int, bool, config.SecondaryNodeConfig, error) {

	const logLoc = "server.checkSpace"

	var nodeConfig config.SecondaryNodeConfig

	vars := mux.Vars(r)
	fileSize := vars["size"]

	intFileSize, err := strconv.Atoi(fileSize)
	if err != nil {
		return 0, false, nodeConfig, logger.CreateDetails(logLoc, err)
	}

	if intFileSize == 0 {
		return 0, false, nodeConfig, logger.CreateDetails(logLoc, err)
	}

	shared.MU.Lock()
	confFile, fileBytes, err := shared.ReadFile(pathToConfig)
	if err != nil {
		return 0, false, nodeConfig, logger.CreateDetails(logLoc, err)
	}
	defer confFile.Close()

	err = json.Unmarshal(fileBytes, &nodeConfig)
	if err != nil {
		return 0, false, nodeConfig, logger.CreateDetails(logLoc, err)
	}

	sharedSpaceInBytes := int64(nodeConfig.StorageLimit) * gbBytes

	nodeConfig.UsedStorageSpace += int64(intFileSize)

	if nodeConfig.UsedStorageSpace > sharedSpaceInBytes {
		return 0, false, nodeConfig, logger.CreateDetails(logLoc, errors.New("not enough space"))
	}

	avaliableSpaceLeft := sharedSpaceInBytes - nodeConfig.UsedStorageSpace

	if avaliableSpaceLeft < oneHunderdMBBytes {
		fmt.Println("Shared storage memory is running low,", avaliableSpaceLeft/(1024*1024), "MB of space is avaliable")
		fmt.Println("You may need additional space for storing data. Total shared space can be changed in account configuration")
	}

	err = config.Save(confFile, nodeConfig)
	if err != nil {
		return 0, false, nodeConfig, logger.CreateDetails(logLoc, err)
	}
	confFile.Close()
	shared.MU.Unlock()

	return intFileSize, true, nodeConfig, nil

}

// ====================================================================================

func parseRequest(r *http.Request) (shared.StorageProviderData, error) {

	const logLoc = "server.parseRequest"

	var spData shared.StorageProviderData

	err := r.ParseMultipartForm(1 << 20) // maxMemory 32MB
	if err != nil {
		return spData, logger.CreateDetails(logLoc, err)
	}

	fs := r.MultipartForm.Value["fs"]

	sort.Strings(fs)

	fsRootHash, fsTree, err := shared.CalcRootHash(fs)
	if err != nil {
		return spData, logger.CreateDetails(logLoc, err)
	}

	signedFsRootHash := r.MultipartForm.Value["fsRootHash"]

	signature, err := hex.DecodeString(signedFsRootHash[0])
	if err != nil {
		return spData, logger.CreateDetails(logLoc, err)
	}

	nonce := r.MultipartForm.Value["nonce"]

	nonceInt, err := strconv.Atoi(nonce[0])
	if err != nil {
		return spData, logger.CreateDetails(logLoc, err)
	}

	nonceHex := strconv.FormatInt(int64(nonceInt), 16)

	nonceBytes, err := hex.DecodeString(nonceHex)
	if err != nil {
		return spData, logger.CreateDetails(logLoc, err)
	}

	nonce32 := make([]byte, 32-len(nonceBytes))
	nonce32 = append(nonce32, nonceBytes...)

	fsRootBytes, err := hex.DecodeString(fsRootHash)
	if err != nil {
		return spData, logger.CreateDetails(logLoc, err)
	}

	fsRootNonceBytes := append(fsRootBytes, nonce32...)

	hash := sha256.Sum256(fsRootNonceBytes)

	sigPublicKey, err := crypto.SigToPub(hash[:], signature)
	if err != nil {
		return spData, logger.CreateDetails(logLoc, err)
	}

	storageProviderAddress := r.MultipartForm.Value["address"]

	senderAddress := crypto.PubkeyToAddress(*sigPublicKey)

	if storageProviderAddress[0] != fmt.Sprint(senderAddress) {
		return spData, logger.CreateDetails(logLoc, errors.New("wrong signature"))
	}

	spData = shared.StorageProviderData{
		Address:      storageProviderAddress[0],
		Fs:           fs,
		Nonce:        nonce[0],
		SignedFsRoot: signedFsRootHash[0],
		Tree:         fsTree,
	}

	return spData, nil
}

// ====================================================================================

func restoreMemoryInfo(pathToConfig string, intFileSize int) {
	logLoc := "server.restoreMemoryInfo->"

	shared.MU.Lock()
	confFile, fileBytes, err := shared.ReadFile(pathToConfig)
	if err != nil {
		shared.MU.Unlock()
		logger.Log(logger.CreateDetails(logLoc, err))
		return
	}
	defer confFile.Close()

	var nodeConfig config.SecondaryNodeConfig

	err = json.Unmarshal(fileBytes, &nodeConfig)
	if err != nil {
		shared.MU.Unlock()
		logger.Log(logger.CreateDetails(logLoc, err))
		return
	}

	nodeConfig.UsedStorageSpace -= int64(intFileSize)

	err = config.Save(confFile, nodeConfig)
	if err != nil {
		shared.MU.Unlock()
		logger.Log(logger.CreateDetails(logLoc, err))
		return
	}
	shared.MU.Unlock()
}

// ====================================================================================

func backUpTo(nodeAddress, addressPath string, multiForm *multipart.Form, fileSize int) (string, error) {
	const logLoc = "server.logLoc->"

	pipeConns := fasthttputil.NewPipeConns()
	pr := pipeConns.Conn1()
	pw := pipeConns.Conn2()

	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()

		address := multiForm.Value["address"]
		err := writer.WriteField("address", address[0])
		if err != nil {
			fmt.Println(err)
			return
		}

		wholeFileHashes := multiForm.Value["fs"]
		for _, wholeHash := range wholeFileHashes {
			err = writer.WriteField("fs", wholeHash)
			if err != nil {
				fmt.Println(err)
				return
			}
		}

		nonce := multiForm.Value["nonce"]
		err = writer.WriteField("nonce", nonce[0])
		if err != nil {
			fmt.Println(err)
			return
		}

		fsRootHash := multiForm.Value["fsRootHash"]
		err = writer.WriteField("fsRootHash", fsRootHash[0])
		if err != nil {
			fmt.Println(err)
			return
		}

		hashes := multiForm.File["hashes"]
		hashesFile, err := hashes[0].Open()
		if err != nil {
			fmt.Println(err)
			return
		}

		defer hashesFile.Close()

		hashesBody, err := io.ReadAll(hashesFile)
		if err != nil {
			fmt.Println(err)
			return
		}

		h, err := writer.CreateFormFile("hashes", "hashes")
		if err != nil {
			fmt.Println(err)
			return
		}

		h.Write(hashesBody)

		hashDif := make(map[string]string)
		err = json.Unmarshal(hashesBody, &hashDif)
		if err != nil {
			fmt.Println(err)
			return
		}

		for old := range hashDif {
			path := filepath.Join(addressPath, old)
			file, err := os.Open(path)
			if err != nil {
				fmt.Println(err)
				return
			}

			defer file.Close()

			filePart, err := writer.CreateFormFile("files", old)
			if err != nil {
				fmt.Println(err)
				return
			}

			_, err = io.Copy(filePart, file)
			if err != nil {
				fmt.Println(err)
				return
			}
		}

		writer.Close()
	}()

	req, err := http.NewRequest("POST", "http://"+nodeAddress+"/backup/"+strconv.Itoa(fileSize), pr)
	if err != nil {
		return "", logger.CreateDetails(logLoc, err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", logger.CreateDetails(logLoc, err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", logger.CreateDetails(logLoc, err)
	}

	defer resp.Body.Close()

	fmt.Println(string(body))
	if string(body) != "OK" {
		return "", logger.CreateDetails(logLoc, errors.New("saving problem"))
	}

	return nodeAddress, nil
}
