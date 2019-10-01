package bitcoind

import (
	"encoding/json"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

// TeslacoilBitcoind is a wrapper around a normal RPC client that
// provides extra functionality used in Teslacoil
type TeslacoilBitcoind interface {
	StartZmq()
	StopZmq()
}

// RpcClient is a client that can query bitcoind/btcd.
type RpcClient interface {

	// All methods below are cribbed from https://godoc.org/github.com/btcsuite/btcd/rpcclient

	AddMultisigAddress(requiredSigs int, addresses []btcutil.Address, account string) (btcutil.Address, error)
	AddMultisigAddressAsync(requiredSigs int, addresses []btcutil.Address, account string) rpcclient.FutureAddMultisigAddressResult
	AddNode(host string, command rpcclient.AddNodeCommand) error
	AddNodeAsync(host string, command rpcclient.AddNodeCommand) rpcclient.FutureAddNodeResult
	AddWitnessAddress(address string) (btcutil.Address, error)
	AddWitnessAddressAsync(address string) rpcclient.FutureAddWitnessAddressResult
	Connect(tries int) error
	CreateEncryptedWallet(passphrase string) error
	CreateEncryptedWalletAsync(passphrase string) rpcclient.FutureCreateEncryptedWalletResult
	CreateMultisig(requiredSigs int, addresses []btcutil.Address) (*btcjson.CreateMultiSigResult, error)
	CreateMultisigAsync(requiredSigs int, addresses []btcutil.Address) rpcclient.FutureCreateMultisigResult
	CreateNewAccount(account string) error
	CreateNewAccountAsync(account string) rpcclient.FutureCreateNewAccountResult
	CreateRawTransaction(inputs []btcjson.TransactionInput, amounts map[btcutil.Address]btcutil.Amount, lockTime *int64) (*wire.MsgTx, error)
	CreateRawTransactionAsync(inputs []btcjson.TransactionInput, amounts map[btcutil.Address]btcutil.Amount, lockTime *int64) rpcclient.FutureCreateRawTransactionResult
	DebugLevel(levelSpec string) (string, error)
	DebugLevelAsync(levelSpec string) rpcclient.FutureDebugLevelResult
	DecodeRawTransaction(serializedTx []byte) (*btcjson.TxRawResult, error)
	DecodeRawTransactionAsync(serializedTx []byte) rpcclient.FutureDecodeRawTransactionResult
	DecodeScript(serializedScript []byte) (*btcjson.DecodeScriptResult, error)
	DecodeScriptAsync(serializedScript []byte) rpcclient.FutureDecodeScriptResult
	Disconnect()
	Disconnected() bool
	DumpPrivKey(address btcutil.Address) (*btcutil.WIF, error)
	DumpPrivKeyAsync(address btcutil.Address) rpcclient.FutureDumpPrivKeyResult
	EstimateFee(numBlocks int64) (float64, error)
	EstimateFeeAsync(numBlocks int64) rpcclient.FutureEstimateFeeResult
	ExportWatchingWallet(account string) ([]byte, []byte, error)
	ExportWatchingWalletAsync(account string) rpcclient.FutureExportWatchingWalletResult
	Generate(numBlocks uint32) ([]*chainhash.Hash, error)
	GenerateAsync(numBlocks uint32) rpcclient.FutureGenerateResult
	GetAccount(address btcutil.Address) (string, error)
	GetAccountAddress(account string) (btcutil.Address, error)
	GetAccountAddressAsync(account string) rpcclient.FutureGetAccountAddressResult
	GetAccountAsync(address btcutil.Address) rpcclient.FutureGetAccountResult
	GetAddedNodeInfo(peer string) ([]btcjson.GetAddedNodeInfoResult, error)
	GetAddedNodeInfoAsync(peer string) rpcclient.FutureGetAddedNodeInfoResult
	GetAddedNodeInfoNoDNS(peer string) ([]string, error)
	GetAddedNodeInfoNoDNSAsync(peer string) rpcclient.FutureGetAddedNodeInfoNoDNSResult
	GetAddressesByAccount(account string) ([]btcutil.Address, error)
	GetAddressesByAccountAsync(account string) rpcclient.FutureGetAddressesByAccountResult
	GetBalance(account string) (btcutil.Amount, error)
	GetBalanceAsync(account string) rpcclient.FutureGetBalanceResult
	GetBalanceMinConf(account string, minConfirms int) (btcutil.Amount, error)
	GetBalanceMinConfAsync(account string, minConfirms int) rpcclient.FutureGetBalanceResult
	GetBestBlock() (*chainhash.Hash, int32, error)
	GetBestBlockAsync() rpcclient.FutureGetBestBlockResult
	GetBestBlockHash() (*chainhash.Hash, error)
	GetBestBlockHashAsync() rpcclient.FutureGetBestBlockHashResult
	GetBlock(blockHash *chainhash.Hash) (*wire.MsgBlock, error)
	GetBlockAsync(blockHash *chainhash.Hash) rpcclient.FutureGetBlockResult
	GetBlockChainInfo() (*btcjson.GetBlockChainInfoResult, error)
	GetBlockChainInfoAsync() rpcclient.FutureGetBlockChainInfoResult
	GetBlockCount() (int64, error)
	GetBlockCountAsync() rpcclient.FutureGetBlockCountResult
	GetBlockHash(blockHeight int64) (*chainhash.Hash, error)
	GetBlockHashAsync(blockHeight int64) rpcclient.FutureGetBlockHashResult
	GetBlockHeader(blockHash *chainhash.Hash) (*wire.BlockHeader, error)
	GetBlockHeaderAsync(blockHash *chainhash.Hash) rpcclient.FutureGetBlockHeaderResult
	GetBlockHeaderVerbose(blockHash *chainhash.Hash) (*btcjson.GetBlockHeaderVerboseResult, error)
	GetBlockHeaderVerboseAsync(blockHash *chainhash.Hash) rpcclient.FutureGetBlockHeaderVerboseResult
	GetBlockVerbose(blockHash *chainhash.Hash) (*btcjson.GetBlockVerboseResult, error)
	GetBlockVerboseAsync(blockHash *chainhash.Hash) rpcclient.FutureGetBlockVerboseResult
	GetBlockVerboseTx(blockHash *chainhash.Hash) (*btcjson.GetBlockVerboseResult, error)
	GetBlockVerboseTxAsync(blockHash *chainhash.Hash) rpcclient.FutureGetBlockVerboseResult
	GetCFilter(blockHash *chainhash.Hash, filterType wire.FilterType) (*wire.MsgCFilter, error)
	GetCFilterAsync(blockHash *chainhash.Hash, filterType wire.FilterType) rpcclient.FutureGetCFilterResult
	GetCFilterHeader(blockHash *chainhash.Hash, filterType wire.FilterType) (*wire.MsgCFHeaders, error)
	GetCFilterHeaderAsync(blockHash *chainhash.Hash, filterType wire.FilterType) rpcclient.FutureGetCFilterHeaderResult
	GetConnectionCount() (int64, error)
	GetConnectionCountAsync() rpcclient.FutureGetConnectionCountResult
	GetCurrentNet() (wire.BitcoinNet, error)
	GetCurrentNetAsync() rpcclient.FutureGetCurrentNetResult
	GetDifficulty() (float64, error)
	GetDifficultyAsync() rpcclient.FutureGetDifficultyResult
	GetGenerate() (bool, error)
	GetGenerateAsync() rpcclient.FutureGetGenerateResult
	GetHashesPerSec() (int64, error)
	GetHashesPerSecAsync() rpcclient.FutureGetHashesPerSecResult
	GetHeaders(blockLocators []chainhash.Hash, hashStop *chainhash.Hash) ([]wire.BlockHeader, error)
	GetHeadersAsync(blockLocators []chainhash.Hash, hashStop *chainhash.Hash) rpcclient.FutureGetHeadersResult
	GetInfo() (*btcjson.InfoWalletResult, error)
	GetInfoAsync() rpcclient.FutureGetInfoResult
	GetMempoolEntry(txHash string) (*btcjson.GetMempoolEntryResult, error)
	GetMempoolEntryAsync(txHash string) rpcclient.FutureGetMempoolEntryResult
	GetMiningInfo() (*btcjson.GetMiningInfoResult, error)
	GetMiningInfoAsync() rpcclient.FutureGetMiningInfoResult
	GetNetTotals() (*btcjson.GetNetTotalsResult, error)
	GetNetTotalsAsync() rpcclient.FutureGetNetTotalsResult
	GetNetworkHashPS() (int64, error)
	GetNetworkHashPS2(blocks int) (int64, error)
	GetNetworkHashPS2Async(blocks int) rpcclient.FutureGetNetworkHashPS
	GetNetworkHashPS3(blocks, height int) (int64, error)
	GetNetworkHashPS3Async(blocks, height int) rpcclient.FutureGetNetworkHashPS
	GetNetworkHashPSAsync() rpcclient.FutureGetNetworkHashPS
	GetNewAddress(account string) (btcutil.Address, error)
	GetNewAddressAsync(account string) rpcclient.FutureGetNewAddressResult
	GetPeerInfo() ([]btcjson.GetPeerInfoResult, error)
	GetPeerInfoAsync() rpcclient.FutureGetPeerInfoResult
	GetRawChangeAddress(account string) (btcutil.Address, error)
	GetRawChangeAddressAsync(account string) rpcclient.FutureGetRawChangeAddressResult
	GetRawMempool() ([]*chainhash.Hash, error)
	GetRawMempoolAsync() rpcclient.FutureGetRawMempoolResult
	GetRawMempoolVerbose() (map[string]btcjson.GetRawMempoolVerboseResult, error)
	GetRawMempoolVerboseAsync() rpcclient.FutureGetRawMempoolVerboseResult
	GetRawTransaction(txHash *chainhash.Hash) (*btcutil.Tx, error)
	GetRawTransactionAsync(txHash *chainhash.Hash) rpcclient.FutureGetRawTransactionResult
	GetRawTransactionVerbose(txHash *chainhash.Hash) (*btcjson.TxRawResult, error)
	GetRawTransactionVerboseAsync(txHash *chainhash.Hash) rpcclient.FutureGetRawTransactionVerboseResult
	GetReceivedByAccount(account string) (btcutil.Amount, error)
	GetReceivedByAccountAsync(account string) rpcclient.FutureGetReceivedByAccountResult
	GetReceivedByAccountMinConf(account string, minConfirms int) (btcutil.Amount, error)
	GetReceivedByAccountMinConfAsync(account string, minConfirms int) rpcclient.FutureGetReceivedByAccountResult
	GetReceivedByAddress(address btcutil.Address) (btcutil.Amount, error)
	GetReceivedByAddressAsync(address btcutil.Address) rpcclient.FutureGetReceivedByAddressResult
	GetReceivedByAddressMinConf(address btcutil.Address, minConfirms int) (btcutil.Amount, error)
	GetReceivedByAddressMinConfAsync(address btcutil.Address, minConfirms int) rpcclient.FutureGetReceivedByAddressResult
	GetTransaction(txHash *chainhash.Hash) (*btcjson.GetTransactionResult, error)
	GetTransactionAsync(txHash *chainhash.Hash) rpcclient.FutureGetTransactionResult
	GetTxOut(txHash *chainhash.Hash, index uint32, mempool bool) (*btcjson.GetTxOutResult, error)
	GetTxOutAsync(txHash *chainhash.Hash, index uint32, mempool bool) rpcclient.FutureGetTxOutResult
	GetUnconfirmedBalance(account string) (btcutil.Amount, error)
	GetUnconfirmedBalanceAsync(account string) rpcclient.FutureGetUnconfirmedBalanceResult
	GetWork() (*btcjson.GetWorkResult, error)
	GetWorkAsync() rpcclient.FutureGetWork
	GetWorkSubmit(data string) (bool, error)
	GetWorkSubmitAsync(data string) rpcclient.FutureGetWorkSubmit
	ImportAddress(address string) error
	ImportAddressAsync(address string) rpcclient.FutureImportAddressResult
	ImportAddressRescan(address string, account string, rescan bool) error
	ImportAddressRescanAsync(address string, account string, rescan bool) rpcclient.FutureImportAddressResult
	ImportPrivKey(privKeyWIF *btcutil.WIF) error
	ImportPrivKeyAsync(privKeyWIF *btcutil.WIF) rpcclient.FutureImportPrivKeyResult
	ImportPrivKeyLabel(privKeyWIF *btcutil.WIF, label string) error
	ImportPrivKeyLabelAsync(privKeyWIF *btcutil.WIF, label string) rpcclient.FutureImportPrivKeyResult
	ImportPrivKeyRescan(privKeyWIF *btcutil.WIF, label string, rescan bool) error
	ImportPrivKeyRescanAsync(privKeyWIF *btcutil.WIF, label string, rescan bool) rpcclient.FutureImportPrivKeyResult
	ImportPubKey(pubKey string) error
	ImportPubKeyAsync(pubKey string) rpcclient.FutureImportPubKeyResult
	ImportPubKeyRescan(pubKey string, rescan bool) error
	ImportPubKeyRescanAsync(pubKey string, rescan bool) rpcclient.FutureImportPubKeyResult
	InvalidateBlock(blockHash *chainhash.Hash) error
	InvalidateBlockAsync(blockHash *chainhash.Hash) rpcclient.FutureInvalidateBlockResult
	KeyPoolRefill() error
	KeyPoolRefillAsync() rpcclient.FutureKeyPoolRefillResult
	KeyPoolRefillSize(newSize uint) error
	KeyPoolRefillSizeAsync(newSize uint) rpcclient.FutureKeyPoolRefillResult
	ListAccounts() (map[string]btcutil.Amount, error)
	ListAccountsAsync() rpcclient.FutureListAccountsResult
	ListAccountsMinConf(minConfirms int) (map[string]btcutil.Amount, error)
	ListAccountsMinConfAsync(minConfirms int) rpcclient.FutureListAccountsResult
	ListAddressTransactions(addresses []btcutil.Address, account string) ([]btcjson.ListTransactionsResult, error)
	ListAddressTransactionsAsync(addresses []btcutil.Address, account string) rpcclient.FutureListAddressTransactionsResult
	ListLockUnspent() ([]*wire.OutPoint, error)
	ListLockUnspentAsync() rpcclient.FutureListLockUnspentResult
	ListReceivedByAccount() ([]btcjson.ListReceivedByAccountResult, error)
	ListReceivedByAccountAsync() rpcclient.FutureListReceivedByAccountResult
	ListReceivedByAccountIncludeEmpty(minConfirms int, includeEmpty bool) ([]btcjson.ListReceivedByAccountResult, error)
	ListReceivedByAccountIncludeEmptyAsync(minConfirms int, includeEmpty bool) rpcclient.FutureListReceivedByAccountResult
	ListReceivedByAccountMinConf(minConfirms int) ([]btcjson.ListReceivedByAccountResult, error)
	ListReceivedByAccountMinConfAsync(minConfirms int) rpcclient.FutureListReceivedByAccountResult
	ListReceivedByAddress() ([]btcjson.ListReceivedByAddressResult, error)
	ListReceivedByAddressAsync() rpcclient.FutureListReceivedByAddressResult
	ListReceivedByAddressIncludeEmpty(minConfirms int, includeEmpty bool) ([]btcjson.ListReceivedByAddressResult, error)
	ListReceivedByAddressIncludeEmptyAsync(minConfirms int, includeEmpty bool) rpcclient.FutureListReceivedByAddressResult
	ListReceivedByAddressMinConf(minConfirms int) ([]btcjson.ListReceivedByAddressResult, error)
	ListReceivedByAddressMinConfAsync(minConfirms int) rpcclient.FutureListReceivedByAddressResult
	ListSinceBlock(blockHash *chainhash.Hash) (*btcjson.ListSinceBlockResult, error)
	ListSinceBlockAsync(blockHash *chainhash.Hash) rpcclient.FutureListSinceBlockResult
	ListSinceBlockMinConf(blockHash *chainhash.Hash, minConfirms int) (*btcjson.ListSinceBlockResult, error)
	ListSinceBlockMinConfAsync(blockHash *chainhash.Hash, minConfirms int) rpcclient.FutureListSinceBlockResult
	ListTransactions(account string) ([]btcjson.ListTransactionsResult, error)
	ListTransactionsAsync(account string) rpcclient.FutureListTransactionsResult
	ListTransactionsCount(account string, count int) ([]btcjson.ListTransactionsResult, error)
	ListTransactionsCountAsync(account string, count int) rpcclient.FutureListTransactionsResult
	ListTransactionsCountFrom(account string, count, from int) ([]btcjson.ListTransactionsResult, error)
	ListTransactionsCountFromAsync(account string, count, from int) rpcclient.FutureListTransactionsResult
	ListUnspent() ([]btcjson.ListUnspentResult, error)
	ListUnspentAsync() rpcclient.FutureListUnspentResult
	ListUnspentMin(minConf int) ([]btcjson.ListUnspentResult, error)
	ListUnspentMinAsync(minConf int) rpcclient.FutureListUnspentResult
	ListUnspentMinMax(minConf, maxConf int) ([]btcjson.ListUnspentResult, error)
	ListUnspentMinMaxAddresses(minConf, maxConf int, addrs []btcutil.Address) ([]btcjson.ListUnspentResult, error)
	ListUnspentMinMaxAddressesAsync(minConf, maxConf int, addrs []btcutil.Address) rpcclient.FutureListUnspentResult
	ListUnspentMinMaxAsync(minConf, maxConf int) rpcclient.FutureListUnspentResult
	LoadTxFilter(reload bool, addresses []btcutil.Address, outPoints []wire.OutPoint) error
	LoadTxFilterAsync(reload bool, addresses []btcutil.Address, outPoints []wire.OutPoint) rpcclient.FutureLoadTxFilterResult
	LockUnspent(unlock bool, ops []*wire.OutPoint) error
	LockUnspentAsync(unlock bool, ops []*wire.OutPoint) rpcclient.FutureLockUnspentResult
	Move(fromAccount, toAccount string, amount btcutil.Amount) (bool, error)
	MoveAsync(fromAccount, toAccount string, amount btcutil.Amount) rpcclient.FutureMoveResult
	MoveComment(fromAccount, toAccount string, amount btcutil.Amount, minConf int, comment string) (bool, error)
	MoveCommentAsync(fromAccount, toAccount string, amount btcutil.Amount, minConfirms int, comment string) rpcclient.FutureMoveResult
	MoveMinConf(fromAccount, toAccount string, amount btcutil.Amount, minConf int) (bool, error)
	MoveMinConfAsync(fromAccount, toAccount string, amount btcutil.Amount, minConfirms int) rpcclient.FutureMoveResult
	NextID() uint64
	Node(command btcjson.NodeSubCmd, host string, connectSubCmd *string) error
	NodeAsync(command btcjson.NodeSubCmd, host string, connectSubCmd *string) rpcclient.FutureNodeResult
	NotifyBlocks() error
	NotifyBlocksAsync() rpcclient.FutureNotifyBlocksResult
	NotifyNewTransactions(verbose bool) error
	NotifyNewTransactionsAsync(verbose bool) rpcclient.FutureNotifyNewTransactionsResult
	NotifyReceived(addresses []btcutil.Address) error
	NotifyReceivedAsync(addresses []btcutil.Address) rpcclient.FutureNotifyReceivedResult
	NotifySpent(outpoints []*wire.OutPoint) error
	NotifySpentAsync(outpoints []*wire.OutPoint) rpcclient.FutureNotifySpentResult
	Ping() error
	PingAsync() rpcclient.FuturePingResult
	RawRequest(method string, params []json.RawMessage) (json.RawMessage, error)
	RawRequestAsync(method string, params []json.RawMessage) rpcclient.FutureRawResult
	RenameAccount(oldAccount, newAccount string) error
	RenameAccountAsync(oldAccount, newAccount string) rpcclient.FutureRenameAccountResult
	Rescan(startBlock *chainhash.Hash, addresses []btcutil.Address, outpoints []*wire.OutPoint) error
	RescanAsync(startBlock *chainhash.Hash, addresses []btcutil.Address, outpoints []*wire.OutPoint) rpcclient.FutureRescanResult
	RescanBlocks(blockHashes []chainhash.Hash) ([]btcjson.RescannedBlock, error)
	RescanBlocksAsync(blockHashes []chainhash.Hash) rpcclient.FutureRescanBlocksResult
	RescanEndBlockAsync(startBlock *chainhash.Hash, addresses []btcutil.Address, outpoints []*wire.OutPoint, endBlock *chainhash.Hash) rpcclient.FutureRescanResult
	RescanEndHeight(startBlock *chainhash.Hash, addresses []btcutil.Address, outpoints []*wire.OutPoint, endBlock *chainhash.Hash) error
	SearchRawTransactions(address btcutil.Address, skip, count int, reverse bool, filterAddrs []string) ([]*wire.MsgTx, error)
	SearchRawTransactionsAsync(address btcutil.Address, skip, count int, reverse bool, filterAddrs []string) rpcclient.FutureSearchRawTransactionsResult
	SearchRawTransactionsVerbose(address btcutil.Address, skip, count int, includePrevOut, reverse bool, filterAddrs []string) ([]*btcjson.SearchRawTransactionsResult, error)
	SearchRawTransactionsVerboseAsync(address btcutil.Address, skip, count int, includePrevOut, reverse bool, filterAddrs *[]string) rpcclient.FutureSearchRawTransactionsVerboseResult
	SendFrom(fromAccount string, toAddress btcutil.Address, amount btcutil.Amount) (*chainhash.Hash, error)
	SendFromAsync(fromAccount string, toAddress btcutil.Address, amount btcutil.Amount) rpcclient.FutureSendFromResult
	SendFromComment(fromAccount string, toAddress btcutil.Address, amount btcutil.Amount, minConfirms int, comment, commentTo string) (*chainhash.Hash, error)
	SendFromCommentAsync(fromAccount string, toAddress btcutil.Address, amount btcutil.Amount, minConfirms int, comment, commentTo string) rpcclient.FutureSendFromResult
	SendFromMinConf(fromAccount string, toAddress btcutil.Address, amount btcutil.Amount, minConfirms int) (*chainhash.Hash, error)
	SendFromMinConfAsync(fromAccount string, toAddress btcutil.Address, amount btcutil.Amount, minConfirms int) rpcclient.FutureSendFromResult
	SendMany(fromAccount string, amounts map[btcutil.Address]btcutil.Amount) (*chainhash.Hash, error)
	SendManyAsync(fromAccount string, amounts map[btcutil.Address]btcutil.Amount) rpcclient.FutureSendManyResult
	SendManyComment(fromAccount string, amounts map[btcutil.Address]btcutil.Amount, minConfirms int, comment string) (*chainhash.Hash, error)
	SendManyCommentAsync(fromAccount string, amounts map[btcutil.Address]btcutil.Amount, minConfirms int, comment string) rpcclient.FutureSendManyResult
	SendManyMinConf(fromAccount string, amounts map[btcutil.Address]btcutil.Amount, minConfirms int) (*chainhash.Hash, error)
	SendManyMinConfAsync(fromAccount string, amounts map[btcutil.Address]btcutil.Amount, minConfirms int) rpcclient.FutureSendManyResult
	SendRawTransaction(tx *wire.MsgTx, allowHighFees bool) (*chainhash.Hash, error)
	SendRawTransactionAsync(tx *wire.MsgTx, allowHighFees bool) rpcclient.FutureSendRawTransactionResult
	SendToAddress(address btcutil.Address, amount btcutil.Amount) (*chainhash.Hash, error)
	SendToAddressAsync(address btcutil.Address, amount btcutil.Amount) rpcclient.FutureSendToAddressResult
	SendToAddressComment(address btcutil.Address, amount btcutil.Amount, comment, commentTo string) (*chainhash.Hash, error)
	SendToAddressCommentAsync(address btcutil.Address, amount btcutil.Amount, comment, commentTo string) rpcclient.FutureSendToAddressResult
	Session() (*btcjson.SessionResult, error)
	SessionAsync() rpcclient.FutureSessionResult
	SetAccount(address btcutil.Address, account string) error
	SetAccountAsync(address btcutil.Address, account string) rpcclient.FutureSetAccountResult
	SetGenerate(enable bool, numCPUs int) error
	SetGenerateAsync(enable bool, numCPUs int) rpcclient.FutureSetGenerateResult
	SetTxFee(fee btcutil.Amount) error
	SetTxFeeAsync(fee btcutil.Amount) rpcclient.FutureSetTxFeeResult
	Shutdown()
	SignMessage(address btcutil.Address, message string) (string, error)
	SignMessageAsync(address btcutil.Address, message string) rpcclient.FutureSignMessageResult
	SignRawTransaction(tx *wire.MsgTx) (*wire.MsgTx, bool, error)
	SignRawTransaction2(tx *wire.MsgTx, inputs []btcjson.RawTxInput) (*wire.MsgTx, bool, error)
	SignRawTransaction2Async(tx *wire.MsgTx, inputs []btcjson.RawTxInput) rpcclient.FutureSignRawTransactionResult
	SignRawTransaction3(tx *wire.MsgTx, inputs []btcjson.RawTxInput, privKeysWIF []string) (*wire.MsgTx, bool, error)
	SignRawTransaction3Async(tx *wire.MsgTx, inputs []btcjson.RawTxInput, privKeysWIF []string) rpcclient.FutureSignRawTransactionResult
	SignRawTransaction4(tx *wire.MsgTx, inputs []btcjson.RawTxInput, privKeysWIF []string, hashType rpcclient.SigHashType) (*wire.MsgTx, bool, error)
	SignRawTransaction4Async(tx *wire.MsgTx, inputs []btcjson.RawTxInput, privKeysWIF []string, hashType rpcclient.SigHashType) rpcclient.FutureSignRawTransactionResult
	SignRawTransactionAsync(tx *wire.MsgTx) rpcclient.FutureSignRawTransactionResult
	SubmitBlock(block *btcutil.Block, options *btcjson.SubmitBlockOptions) error
	SubmitBlockAsync(block *btcutil.Block, options *btcjson.SubmitBlockOptions) rpcclient.FutureSubmitBlockResult
	ValidateAddress(address btcutil.Address) (*btcjson.ValidateAddressWalletResult, error)
	ValidateAddressAsync(address btcutil.Address) rpcclient.FutureValidateAddressResult
	VerifyChain() (bool, error)
	VerifyChainAsync() rpcclient.FutureVerifyChainResult
	VerifyChainBlocks(checkLevel, numBlocks int32) (bool, error)
	VerifyChainBlocksAsync(checkLevel, numBlocks int32) rpcclient.FutureVerifyChainResult
	VerifyChainLevel(checkLevel int32) (bool, error)
	VerifyChainLevelAsync(checkLevel int32) rpcclient.FutureVerifyChainResult
	VerifyMessage(address btcutil.Address, signature, message string) (bool, error)
	VerifyMessageAsync(address btcutil.Address, signature, message string) rpcclient.FutureVerifyMessageResult
	Version() (map[string]btcjson.VersionResult, error)
	VersionAsync() rpcclient.FutureVersionResult
	WaitForShutdown()
	WalletLock() error
	WalletLockAsync() rpcclient.FutureWalletLockResult
	WalletPassphrase(passphrase string, timeoutSecs int64) error
	WalletPassphraseChange(old, new string) error
	WalletPassphraseChangeAsync(old, new string) rpcclient.FutureWalletPassphraseChangeResult
}
