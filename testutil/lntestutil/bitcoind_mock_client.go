package lntestutil

import (
	"encoding/json"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/bitcoind"
)

var (
	// check that mock clients satisfies interfaces
	_ bitcoind.RpcClient         = BitcoindRpcMockClient{}
	_ bitcoind.TeslacoilBitcoind = TeslacoilBitcoindMockClient{}
)

// TeslacoilBitcoindMockClient is a mocked out bitcoind.TeslacoilBitcoind
// where the responses don't contain any meaningful info at all.
type TeslacoilBitcoindMockClient struct{}

func (t TeslacoilBitcoindMockClient) StartZmq() {}

func (t TeslacoilBitcoindMockClient) StopZmq() {}

func (t TeslacoilBitcoindMockClient) GetZmqRawTxChannel() chan *wire.MsgTx {
	return make(chan *wire.MsgTx)
}

func (t TeslacoilBitcoindMockClient) GetZmqRawBlockChannel() chan *wire.MsgBlock {
	return make(chan *wire.MsgBlock)
}

func (t TeslacoilBitcoindMockClient) Client() bitcoind.RpcClient {
	return BitcoindRpcMockClient{}
}

// BitcoindRpcMockClient is a mocked out bitcoind.RpcClient where the responses
// don't contain any meaningful info at all.
type BitcoindRpcMockClient struct{}

func (b BitcoindRpcMockClient) AddMultisigAddress(requiredSigs int, addresses []btcutil.Address, account string) (btcutil.Address, error) {
	panic("not implemented: AddMultisigAddress")
}

func (b BitcoindRpcMockClient) AddMultisigAddressAsync(requiredSigs int, addresses []btcutil.Address, account string) rpcclient.FutureAddMultisigAddressResult {
	panic("not implemented: AddMultisigAddressAsync")
}

func (b BitcoindRpcMockClient) AddNode(host string, command rpcclient.AddNodeCommand) error {
	panic("not implemented: AddNode")
}

func (b BitcoindRpcMockClient) AddNodeAsync(host string, command rpcclient.AddNodeCommand) rpcclient.FutureAddNodeResult {
	panic("not implemented: AddNodeAsync")
}

func (b BitcoindRpcMockClient) AddWitnessAddress(address string) (btcutil.Address, error) {
	panic("not implemented: AddWitnessAddress")
}

func (b BitcoindRpcMockClient) AddWitnessAddressAsync(address string) rpcclient.FutureAddWitnessAddressResult {
	panic("not implemented: AddWitnessAddressAsync")
}

func (b BitcoindRpcMockClient) Connect(tries int) error {
	panic("not implemented: Connect")
}

func (b BitcoindRpcMockClient) CreateEncryptedWallet(passphrase string) error {
	panic("not implemented: CreateEncryptedWallet")
}

func (b BitcoindRpcMockClient) CreateEncryptedWalletAsync(passphrase string) rpcclient.FutureCreateEncryptedWalletResult {
	panic("not implemented: CreateEncryptedWalletAsync")
}

func (b BitcoindRpcMockClient) CreateMultisig(requiredSigs int, addresses []btcutil.Address) (*btcjson.CreateMultiSigResult, error) {
	panic("not implemented: CreateMultisig")
}

func (b BitcoindRpcMockClient) CreateMultisigAsync(requiredSigs int, addresses []btcutil.Address) rpcclient.FutureCreateMultisigResult {
	panic("not implemented: CreateMultisigAsync")
}

func (b BitcoindRpcMockClient) CreateNewAccount(account string) error {
	panic("not implemented: CreateNewAccount")
}

func (b BitcoindRpcMockClient) CreateNewAccountAsync(account string) rpcclient.FutureCreateNewAccountResult {
	panic("not implemented: CreateNewAccountAsync")
}

func (b BitcoindRpcMockClient) CreateRawTransaction(inputs []btcjson.TransactionInput, amounts map[btcutil.Address]btcutil.Amount, lockTime *int64) (*wire.MsgTx, error) {
	panic("not implemented: CreateRawTransaction")
}

func (b BitcoindRpcMockClient) CreateRawTransactionAsync(inputs []btcjson.TransactionInput, amounts map[btcutil.Address]btcutil.Amount, lockTime *int64) rpcclient.FutureCreateRawTransactionResult {
	panic("not implemented: CreateRawTransactionAsync")
}

func (b BitcoindRpcMockClient) DebugLevel(levelSpec string) (string, error) {
	panic("not implemented: DebugLevel")
}

func (b BitcoindRpcMockClient) DebugLevelAsync(levelSpec string) rpcclient.FutureDebugLevelResult {
	panic("not implemented: DebugLevelAsync")
}

func (b BitcoindRpcMockClient) DecodeRawTransaction(serializedTx []byte) (*btcjson.TxRawResult, error) {
	panic("not implemented: DecodeRawTransaction")
}

func (b BitcoindRpcMockClient) DecodeRawTransactionAsync(serializedTx []byte) rpcclient.FutureDecodeRawTransactionResult {
	panic("not implemented: DecodeRawTransactionAsync")
}

func (b BitcoindRpcMockClient) DecodeScript(serializedScript []byte) (*btcjson.DecodeScriptResult, error) {
	panic("not implemented: DecodeScript")
}

func (b BitcoindRpcMockClient) DecodeScriptAsync(serializedScript []byte) rpcclient.FutureDecodeScriptResult {
	panic("not implemented: DecodeScriptAsync")
}

func (b BitcoindRpcMockClient) Disconnect() {
	panic("not implemented: Disconnect")
}

func (b BitcoindRpcMockClient) Disconnected() bool {
	panic("not implemented: Disconnected")
}

func (b BitcoindRpcMockClient) DumpPrivKey(address btcutil.Address) (*btcutil.WIF, error) {
	panic("not implemented: DumpPrivKey")
}

func (b BitcoindRpcMockClient) DumpPrivKeyAsync(address btcutil.Address) rpcclient.FutureDumpPrivKeyResult {
	panic("not implemented: DumpPrivKeyAsync")
}

func (b BitcoindRpcMockClient) EstimateFee(numBlocks int64) (float64, error) {
	panic("not implemented: EstimateFee")
}

func (b BitcoindRpcMockClient) EstimateFeeAsync(numBlocks int64) rpcclient.FutureEstimateFeeResult {
	panic("not implemented: EstimateFeeAsync")
}

func (b BitcoindRpcMockClient) ExportWatchingWallet(account string) ([]byte, []byte, error) {
	panic("not implemented: ExportWatchingWallet")
}

func (b BitcoindRpcMockClient) ExportWatchingWalletAsync(account string) rpcclient.FutureExportWatchingWalletResult {
	panic("not implemented: ExportWatchingWalletAsync")
}

func (b BitcoindRpcMockClient) Generate(numBlocks uint32) ([]*chainhash.Hash, error) {
	panic("not implemented: Generate")
}

func (b BitcoindRpcMockClient) GenerateAsync(numBlocks uint32) rpcclient.FutureGenerateResult {
	panic("not implemented: GenerateAsync")
}

func (b BitcoindRpcMockClient) GetAccount(address btcutil.Address) (string, error) {
	panic("not implemented: GetAccount")
}

func (b BitcoindRpcMockClient) GetAccountAddress(account string) (btcutil.Address, error) {
	panic("not implemented: GetAccountAddress")
}

func (b BitcoindRpcMockClient) GetAccountAddressAsync(account string) rpcclient.FutureGetAccountAddressResult {
	panic("not implemented: GetAccountAddressAsync")
}

func (b BitcoindRpcMockClient) GetAccountAsync(address btcutil.Address) rpcclient.FutureGetAccountResult {
	panic("not implemented: GetAccountAsync")
}

func (b BitcoindRpcMockClient) GetAddedNodeInfo(peer string) ([]btcjson.GetAddedNodeInfoResult, error) {
	panic("not implemented: GetAddedNodeInfo")
}

func (b BitcoindRpcMockClient) GetAddedNodeInfoAsync(peer string) rpcclient.FutureGetAddedNodeInfoResult {
	panic("not implemented: GetAddedNodeInfoAsync")
}

func (b BitcoindRpcMockClient) GetAddedNodeInfoNoDNS(peer string) ([]string, error) {
	panic("not implemented: GetAddedNodeInfoNoDNS")
}

func (b BitcoindRpcMockClient) GetAddedNodeInfoNoDNSAsync(peer string) rpcclient.FutureGetAddedNodeInfoNoDNSResult {
	panic("not implemented: GetAddedNodeInfoNoDNSAsync")
}

func (b BitcoindRpcMockClient) GetAddressesByAccount(account string) ([]btcutil.Address, error) {
	panic("not implemented: GetAddressesByAccount")
}

func (b BitcoindRpcMockClient) GetAddressesByAccountAsync(account string) rpcclient.FutureGetAddressesByAccountResult {
	panic("not implemented: GetAddressesByAccountAsync")
}

func (b BitcoindRpcMockClient) GetBalance(account string) (btcutil.Amount, error) {
	panic("not implemented: GetBalance")
}

func (b BitcoindRpcMockClient) GetBalanceAsync(account string) rpcclient.FutureGetBalanceResult {
	panic("not implemented: GetBalanceAsync")
}

func (b BitcoindRpcMockClient) GetBalanceMinConf(account string, minConfirms int) (btcutil.Amount, error) {
	panic("not implemented: GetBalanceMinConf")
}

func (b BitcoindRpcMockClient) GetBalanceMinConfAsync(account string, minConfirms int) rpcclient.FutureGetBalanceResult {
	panic("not implemented: GetBalanceMinConfAsync")
}

func (b BitcoindRpcMockClient) GetBestBlock() (*chainhash.Hash, int32, error) {
	panic("not implemented: GetBestBlock")
}

func (b BitcoindRpcMockClient) GetBestBlockAsync() rpcclient.FutureGetBestBlockResult {
	panic("not implemented: GetBestBlockAsync")
}

func (b BitcoindRpcMockClient) GetBestBlockHash() (*chainhash.Hash, error) {
	panic("not implemented: GetBestBlockHash")
}

func (b BitcoindRpcMockClient) GetBestBlockHashAsync() rpcclient.FutureGetBestBlockHashResult {
	panic("not implemented: GetBestBlockHashAsync")
}

func (b BitcoindRpcMockClient) GetBlock(blockHash *chainhash.Hash) (*wire.MsgBlock, error) {
	panic("not implemented: GetBlock")
}

func (b BitcoindRpcMockClient) GetBlockAsync(blockHash *chainhash.Hash) rpcclient.FutureGetBlockResult {
	panic("not implemented: GetBlockAsync")
}

func (b BitcoindRpcMockClient) GetBlockChainInfo() (*btcjson.GetBlockChainInfoResult, error) {
	return &btcjson.GetBlockChainInfoResult{
		Chain:                "regtes",
		Blocks:               0,
		Headers:              0,
		BestBlockHash:        "",
		Difficulty:           0,
		MedianTime:           0,
		VerificationProgress: 0,
		Pruned:               false,
		PruneHeight:          0,
		ChainWork:            "",
		SoftForks:            nil,
		Bip9SoftForks:        nil,
	}, nil
}

func (b BitcoindRpcMockClient) GetBlockChainInfoAsync() rpcclient.FutureGetBlockChainInfoResult {
	panic("not implemented: GetBlockChainInfoAsync")
}

func (b BitcoindRpcMockClient) GetBlockCount() (int64, error) {
	panic("not implemented: GetBlockCount")
}

func (b BitcoindRpcMockClient) GetBlockCountAsync() rpcclient.FutureGetBlockCountResult {
	panic("not implemented: GetBlockCountAsync")
}

func (b BitcoindRpcMockClient) GetBlockHash(blockHeight int64) (*chainhash.Hash, error) {
	panic("not implemented: GetBlockHash")
}

func (b BitcoindRpcMockClient) GetBlockHashAsync(blockHeight int64) rpcclient.FutureGetBlockHashResult {
	panic("not implemented: GetBlockHashAsync")
}

func (b BitcoindRpcMockClient) GetBlockHeader(blockHash *chainhash.Hash) (*wire.BlockHeader, error) {
	panic("not implemented: GetBlockHeader")
}

func (b BitcoindRpcMockClient) GetBlockHeaderAsync(blockHash *chainhash.Hash) rpcclient.FutureGetBlockHeaderResult {
	panic("not implemented: GetBlockHeaderAsync")
}

func (b BitcoindRpcMockClient) GetBlockHeaderVerbose(blockHash *chainhash.Hash) (*btcjson.GetBlockHeaderVerboseResult, error) {
	panic("not implemented: GetBlockHeaderVerbose")
}

func (b BitcoindRpcMockClient) GetBlockHeaderVerboseAsync(blockHash *chainhash.Hash) rpcclient.FutureGetBlockHeaderVerboseResult {
	panic("not implemented: GetBlockHeaderVerboseAsync")
}

func (b BitcoindRpcMockClient) GetBlockVerbose(blockHash *chainhash.Hash) (*btcjson.GetBlockVerboseResult, error) {
	panic("not implemented: GetBlockVerbose")
}

func (b BitcoindRpcMockClient) GetBlockVerboseAsync(blockHash *chainhash.Hash) rpcclient.FutureGetBlockVerboseResult {
	panic("not implemented: GetBlockVerboseAsync")
}

func (b BitcoindRpcMockClient) GetBlockVerboseTx(blockHash *chainhash.Hash) (*btcjson.GetBlockVerboseResult, error) {
	panic("not implemented: GetBlockVerboseTx")
}

func (b BitcoindRpcMockClient) GetBlockVerboseTxAsync(blockHash *chainhash.Hash) rpcclient.FutureGetBlockVerboseResult {
	panic("not implemented: GetBlockVerboseTxAsync")
}

func (b BitcoindRpcMockClient) GetCFilter(blockHash *chainhash.Hash, filterType wire.FilterType) (*wire.MsgCFilter, error) {
	panic("not implemented: GetCFilter")
}

func (b BitcoindRpcMockClient) GetCFilterAsync(blockHash *chainhash.Hash, filterType wire.FilterType) rpcclient.FutureGetCFilterResult {
	panic("not implemented: GetCFilterAsync")
}

func (b BitcoindRpcMockClient) GetCFilterHeader(blockHash *chainhash.Hash, filterType wire.FilterType) (*wire.MsgCFHeaders, error) {
	panic("not implemented: GetCFilterHeader")
}

func (b BitcoindRpcMockClient) GetCFilterHeaderAsync(blockHash *chainhash.Hash, filterType wire.FilterType) rpcclient.FutureGetCFilterHeaderResult {
	panic("not implemented: GetCFilterHeaderAsync")
}

func (b BitcoindRpcMockClient) GetConnectionCount() (int64, error) {
	panic("not implemented: GetConnectionCount")
}

func (b BitcoindRpcMockClient) GetConnectionCountAsync() rpcclient.FutureGetConnectionCountResult {
	panic("not implemented: GetConnectionCountAsync")
}

func (b BitcoindRpcMockClient) GetCurrentNet() (wire.BitcoinNet, error) {
	panic("not implemented: GetCurrentNet")
}

func (b BitcoindRpcMockClient) GetCurrentNetAsync() rpcclient.FutureGetCurrentNetResult {
	panic("not implemented: GetCurrentNetAsync")
}

func (b BitcoindRpcMockClient) GetDifficulty() (float64, error) {
	panic("not implemented: GetDifficulty")
}

func (b BitcoindRpcMockClient) GetDifficultyAsync() rpcclient.FutureGetDifficultyResult {
	panic("not implemented: GetDifficultyAsync")
}

func (b BitcoindRpcMockClient) GetGenerate() (bool, error) {
	panic("not implemented: GetGenerate")
}

func (b BitcoindRpcMockClient) GetGenerateAsync() rpcclient.FutureGetGenerateResult {
	panic("not implemented: GetGenerateAsync")
}

func (b BitcoindRpcMockClient) GetHashesPerSec() (int64, error) {
	panic("not implemented: GetHashesPerSec")
}

func (b BitcoindRpcMockClient) GetHashesPerSecAsync() rpcclient.FutureGetHashesPerSecResult {
	panic("not implemented: GetHashesPerSecAsync")
}

func (b BitcoindRpcMockClient) GetHeaders(blockLocators []chainhash.Hash, hashStop *chainhash.Hash) ([]wire.BlockHeader, error) {
	panic("not implemented: GetHeaders")
}

func (b BitcoindRpcMockClient) GetHeadersAsync(blockLocators []chainhash.Hash, hashStop *chainhash.Hash) rpcclient.FutureGetHeadersResult {
	panic("not implemented: GetHeadersAsync")
}

func (b BitcoindRpcMockClient) GetInfo() (*btcjson.InfoWalletResult, error) {
	panic("not implemented: GetInfo")
}

func (b BitcoindRpcMockClient) GetInfoAsync() rpcclient.FutureGetInfoResult {
	panic("not implemented: GetInfoAsync")
}

func (b BitcoindRpcMockClient) GetMempoolEntry(txHash string) (*btcjson.GetMempoolEntryResult, error) {
	panic("not implemented: GetMempoolEntry")
}

func (b BitcoindRpcMockClient) GetMempoolEntryAsync(txHash string) rpcclient.FutureGetMempoolEntryResult {
	panic("not implemented: GetMempoolEntryAsync")
}

func (b BitcoindRpcMockClient) GetMiningInfo() (*btcjson.GetMiningInfoResult, error) {
	panic("not implemented: GetMiningInfo")
}

func (b BitcoindRpcMockClient) GetMiningInfoAsync() rpcclient.FutureGetMiningInfoResult {
	panic("not implemented: GetMiningInfoAsync")
}

func (b BitcoindRpcMockClient) GetNetTotals() (*btcjson.GetNetTotalsResult, error) {
	panic("not implemented: GetNetTotals")
}

func (b BitcoindRpcMockClient) GetNetTotalsAsync() rpcclient.FutureGetNetTotalsResult {
	panic("not implemented: GetNetTotalsAsync")
}

func (b BitcoindRpcMockClient) GetNetworkHashPS() (int64, error) {
	panic("not implemented: GetNetworkHashPS")
}

func (b BitcoindRpcMockClient) GetNetworkHashPS2(blocks int) (int64, error) {
	panic("not implemented: GetNetworkHashPS2")
}

func (b BitcoindRpcMockClient) GetNetworkHashPS2Async(blocks int) rpcclient.FutureGetNetworkHashPS {
	panic("not implemented: GetNetworkHashPS2Async")
}

func (b BitcoindRpcMockClient) GetNetworkHashPS3(blocks, height int) (int64, error) {
	panic("not implemented: GetNetworkHashPS3")
}

func (b BitcoindRpcMockClient) GetNetworkHashPS3Async(blocks, height int) rpcclient.FutureGetNetworkHashPS {
	panic("not implemented: GetNetworkHashPS3Async")
}

func (b BitcoindRpcMockClient) GetNetworkHashPSAsync() rpcclient.FutureGetNetworkHashPS {
	panic("not implemented: GetNetworkHashPSAsync")
}

func (b BitcoindRpcMockClient) GetNewAddress(account string) (btcutil.Address, error) {
	panic("not implemented: GetNewAddress")
}

func (b BitcoindRpcMockClient) GetNewAddressAsync(account string) rpcclient.FutureGetNewAddressResult {
	panic("not implemented: GetNewAddressAsync")
}

func (b BitcoindRpcMockClient) GetPeerInfo() ([]btcjson.GetPeerInfoResult, error) {
	panic("not implemented: GetPeerInfo")
}

func (b BitcoindRpcMockClient) GetPeerInfoAsync() rpcclient.FutureGetPeerInfoResult {
	panic("not implemented: GetPeerInfoAsync")
}

func (b BitcoindRpcMockClient) GetRawChangeAddress(account string) (btcutil.Address, error) {
	panic("not implemented: GetRawChangeAddress")
}

func (b BitcoindRpcMockClient) GetRawChangeAddressAsync(account string) rpcclient.FutureGetRawChangeAddressResult {
	panic("not implemented: GetRawChangeAddressAsync")
}

func (b BitcoindRpcMockClient) GetRawMempool() ([]*chainhash.Hash, error) {
	panic("not implemented: GetRawMempool")
}

func (b BitcoindRpcMockClient) GetRawMempoolAsync() rpcclient.FutureGetRawMempoolResult {
	panic("not implemented: GetRawMempoolAsync")
}

func (b BitcoindRpcMockClient) GetRawMempoolVerbose() (map[string]btcjson.GetRawMempoolVerboseResult, error) {
	panic("not implemented: GetRawMempoolVerbose")
}

func (b BitcoindRpcMockClient) GetRawMempoolVerboseAsync() rpcclient.FutureGetRawMempoolVerboseResult {
	panic("not implemented: GetRawMempoolVerboseAsync")
}

func (b BitcoindRpcMockClient) GetRawTransaction(txHash *chainhash.Hash) (*btcutil.Tx, error) {
	panic("not implemented: GetRawTransaction")
}

func (b BitcoindRpcMockClient) GetRawTransactionAsync(txHash *chainhash.Hash) rpcclient.FutureGetRawTransactionResult {
	panic("not implemented: GetRawTransactionAsync")
}

func (b BitcoindRpcMockClient) GetRawTransactionVerbose(txHash *chainhash.Hash) (*btcjson.TxRawResult, error) {
	panic("not implemented: GetRawTransactionVerbose")
}

func (b BitcoindRpcMockClient) GetRawTransactionVerboseAsync(txHash *chainhash.Hash) rpcclient.FutureGetRawTransactionVerboseResult {
	panic("not implemented: GetRawTransactionVerboseAsync")
}

func (b BitcoindRpcMockClient) GetReceivedByAccount(account string) (btcutil.Amount, error) {
	panic("not implemented: GetReceivedByAccount")
}

func (b BitcoindRpcMockClient) GetReceivedByAccountAsync(account string) rpcclient.FutureGetReceivedByAccountResult {
	panic("not implemented: GetReceivedByAccountAsync")
}

func (b BitcoindRpcMockClient) GetReceivedByAccountMinConf(account string, minConfirms int) (btcutil.Amount, error) {
	panic("not implemented: GetReceivedByAccountMinConf")
}

func (b BitcoindRpcMockClient) GetReceivedByAccountMinConfAsync(account string, minConfirms int) rpcclient.FutureGetReceivedByAccountResult {
	panic("not implemented: GetReceivedByAccountMinConfAsync")
}

func (b BitcoindRpcMockClient) GetReceivedByAddress(address btcutil.Address) (btcutil.Amount, error) {
	panic("not implemented: GetReceivedByAddress")
}

func (b BitcoindRpcMockClient) GetReceivedByAddressAsync(address btcutil.Address) rpcclient.FutureGetReceivedByAddressResult {
	panic("not implemented: GetReceivedByAddressAsync")
}

func (b BitcoindRpcMockClient) GetReceivedByAddressMinConf(address btcutil.Address, minConfirms int) (btcutil.Amount, error) {
	panic("not implemented: GetReceivedByAddressMinConf")
}

func (b BitcoindRpcMockClient) GetReceivedByAddressMinConfAsync(address btcutil.Address, minConfirms int) rpcclient.FutureGetReceivedByAddressResult {
	panic("not implemented: GetReceivedByAddressMinConfAsync")
}

func (b BitcoindRpcMockClient) GetTransaction(txHash *chainhash.Hash) (*btcjson.GetTransactionResult, error) {
	panic("not implemented: GetTransaction")
}

func (b BitcoindRpcMockClient) GetTransactionAsync(txHash *chainhash.Hash) rpcclient.FutureGetTransactionResult {
	panic("not implemented: GetTransactionAsync")
}

func (b BitcoindRpcMockClient) GetTxOut(txHash *chainhash.Hash, index uint32, mempool bool) (*btcjson.GetTxOutResult, error) {
	panic("not implemented: GetTxOut")
}

func (b BitcoindRpcMockClient) GetTxOutAsync(txHash *chainhash.Hash, index uint32, mempool bool) rpcclient.FutureGetTxOutResult {
	panic("not implemented: GetTxOutAsync")
}

func (b BitcoindRpcMockClient) GetUnconfirmedBalance(account string) (btcutil.Amount, error) {
	panic("not implemented: GetUnconfirmedBalance")
}

func (b BitcoindRpcMockClient) GetUnconfirmedBalanceAsync(account string) rpcclient.FutureGetUnconfirmedBalanceResult {
	panic("not implemented: GetUnconfirmedBalanceAsync")
}

func (b BitcoindRpcMockClient) GetWork() (*btcjson.GetWorkResult, error) {
	panic("not implemented: GetWork")
}

func (b BitcoindRpcMockClient) GetWorkAsync() rpcclient.FutureGetWork {
	panic("not implemented: GetWorkAsync")
}

func (b BitcoindRpcMockClient) GetWorkSubmit(data string) (bool, error) {
	panic("not implemented: GetWorkSubmit")
}

func (b BitcoindRpcMockClient) GetWorkSubmitAsync(data string) rpcclient.FutureGetWorkSubmit {
	panic("not implemented: GetWorkSubmitAsync")
}

func (b BitcoindRpcMockClient) ImportAddress(address string) error {
	panic("not implemented: ImportAddress")
}

func (b BitcoindRpcMockClient) ImportAddressAsync(address string) rpcclient.FutureImportAddressResult {
	panic("not implemented: ImportAddressAsync")
}

func (b BitcoindRpcMockClient) ImportAddressRescan(address string, account string, rescan bool) error {
	panic("not implemented: ImportAddressRescan")
}

func (b BitcoindRpcMockClient) ImportAddressRescanAsync(address string, account string, rescan bool) rpcclient.FutureImportAddressResult {
	panic("not implemented: ImportAddressRescanAsync")
}

func (b BitcoindRpcMockClient) ImportPrivKey(privKeyWIF *btcutil.WIF) error {
	panic("not implemented: ImportPrivKey")
}

func (b BitcoindRpcMockClient) ImportPrivKeyAsync(privKeyWIF *btcutil.WIF) rpcclient.FutureImportPrivKeyResult {
	panic("not implemented: ImportPrivKeyAsync")
}

func (b BitcoindRpcMockClient) ImportPrivKeyLabel(privKeyWIF *btcutil.WIF, label string) error {
	panic("not implemented: ImportPrivKeyLabel")
}

func (b BitcoindRpcMockClient) ImportPrivKeyLabelAsync(privKeyWIF *btcutil.WIF, label string) rpcclient.FutureImportPrivKeyResult {
	panic("not implemented: ImportPrivKeyLabelAsync")
}

func (b BitcoindRpcMockClient) ImportPrivKeyRescan(privKeyWIF *btcutil.WIF, label string, rescan bool) error {
	panic("not implemented: ImportPrivKeyRescan")
}

func (b BitcoindRpcMockClient) ImportPrivKeyRescanAsync(privKeyWIF *btcutil.WIF, label string, rescan bool) rpcclient.FutureImportPrivKeyResult {
	panic("not implemented: ImportPrivKeyRescanAsync")
}

func (b BitcoindRpcMockClient) ImportPubKey(pubKey string) error {
	panic("not implemented: ImportPubKey")
}

func (b BitcoindRpcMockClient) ImportPubKeyAsync(pubKey string) rpcclient.FutureImportPubKeyResult {
	panic("not implemented: ImportPubKeyAsync")
}

func (b BitcoindRpcMockClient) ImportPubKeyRescan(pubKey string, rescan bool) error {
	panic("not implemented: ImportPubKeyRescan")
}

func (b BitcoindRpcMockClient) ImportPubKeyRescanAsync(pubKey string, rescan bool) rpcclient.FutureImportPubKeyResult {
	panic("not implemented: ImportPubKeyRescanAsync")
}

func (b BitcoindRpcMockClient) InvalidateBlock(blockHash *chainhash.Hash) error {
	panic("not implemented: InvalidateBlock")
}

func (b BitcoindRpcMockClient) InvalidateBlockAsync(blockHash *chainhash.Hash) rpcclient.FutureInvalidateBlockResult {
	panic("not implemented: InvalidateBlockAsync")
}

func (b BitcoindRpcMockClient) KeyPoolRefill() error {
	panic("not implemented: KeyPoolRefill")
}

func (b BitcoindRpcMockClient) KeyPoolRefillAsync() rpcclient.FutureKeyPoolRefillResult {
	panic("not implemented: KeyPoolRefillAsync")
}

func (b BitcoindRpcMockClient) KeyPoolRefillSize(newSize uint) error {
	panic("not implemented: KeyPoolRefillSize")
}

func (b BitcoindRpcMockClient) KeyPoolRefillSizeAsync(newSize uint) rpcclient.FutureKeyPoolRefillResult {
	panic("not implemented: KeyPoolRefillSizeAsync")
}

func (b BitcoindRpcMockClient) ListAccounts() (map[string]btcutil.Amount, error) {
	panic("not implemented: ListAccounts")
}

func (b BitcoindRpcMockClient) ListAccountsAsync() rpcclient.FutureListAccountsResult {
	panic("not implemented: ListAccountsAsync")
}

func (b BitcoindRpcMockClient) ListAccountsMinConf(minConfirms int) (map[string]btcutil.Amount, error) {
	panic("not implemented: ListAccountsMinConf")
}

func (b BitcoindRpcMockClient) ListAccountsMinConfAsync(minConfirms int) rpcclient.FutureListAccountsResult {
	panic("not implemented: ListAccountsMinConfAsync")
}

func (b BitcoindRpcMockClient) ListAddressTransactions(addresses []btcutil.Address, account string) ([]btcjson.ListTransactionsResult, error) {
	panic("not implemented: ListAddressTransactions")
}

func (b BitcoindRpcMockClient) ListAddressTransactionsAsync(addresses []btcutil.Address, account string) rpcclient.FutureListAddressTransactionsResult {
	panic("not implemented: ListAddressTransactionsAsync")
}

func (b BitcoindRpcMockClient) ListLockUnspent() ([]*wire.OutPoint, error) {
	panic("not implemented: ListLockUnspent")
}

func (b BitcoindRpcMockClient) ListLockUnspentAsync() rpcclient.FutureListLockUnspentResult {
	panic("not implemented: ListLockUnspentAsync")
}

func (b BitcoindRpcMockClient) ListReceivedByAccount() ([]btcjson.ListReceivedByAccountResult, error) {
	panic("not implemented: ListReceivedByAccount")
}

func (b BitcoindRpcMockClient) ListReceivedByAccountAsync() rpcclient.FutureListReceivedByAccountResult {
	panic("not implemented: ListReceivedByAccountAsync")
}

func (b BitcoindRpcMockClient) ListReceivedByAccountIncludeEmpty(minConfirms int, includeEmpty bool) ([]btcjson.ListReceivedByAccountResult, error) {
	panic("not implemented: ListReceivedByAccountIncludeEmpty")
}

func (b BitcoindRpcMockClient) ListReceivedByAccountIncludeEmptyAsync(minConfirms int, includeEmpty bool) rpcclient.FutureListReceivedByAccountResult {
	panic("not implemented: ListReceivedByAccountIncludeEmptyAsync")
}

func (b BitcoindRpcMockClient) ListReceivedByAccountMinConf(minConfirms int) ([]btcjson.ListReceivedByAccountResult, error) {
	panic("not implemented: ListReceivedByAccountMinConf")
}

func (b BitcoindRpcMockClient) ListReceivedByAccountMinConfAsync(minConfirms int) rpcclient.FutureListReceivedByAccountResult {
	panic("not implemented: ListReceivedByAccountMinConfAsync")
}

func (b BitcoindRpcMockClient) ListReceivedByAddress() ([]btcjson.ListReceivedByAddressResult, error) {
	panic("not implemented: ListReceivedByAddress")
}

func (b BitcoindRpcMockClient) ListReceivedByAddressAsync() rpcclient.FutureListReceivedByAddressResult {
	panic("not implemented: ListReceivedByAddressAsync")
}

func (b BitcoindRpcMockClient) ListReceivedByAddressIncludeEmpty(minConfirms int, includeEmpty bool) ([]btcjson.ListReceivedByAddressResult, error) {
	panic("not implemented: ListReceivedByAddressIncludeEmpty")
}

func (b BitcoindRpcMockClient) ListReceivedByAddressIncludeEmptyAsync(minConfirms int, includeEmpty bool) rpcclient.FutureListReceivedByAddressResult {
	panic("not implemented: ListReceivedByAddressIncludeEmptyAsync")
}

func (b BitcoindRpcMockClient) ListReceivedByAddressMinConf(minConfirms int) ([]btcjson.ListReceivedByAddressResult, error) {
	panic("not implemented: ListReceivedByAddressMinConf")
}

func (b BitcoindRpcMockClient) ListReceivedByAddressMinConfAsync(minConfirms int) rpcclient.FutureListReceivedByAddressResult {
	panic("not implemented: ListReceivedByAddressMinConfAsync")
}

func (b BitcoindRpcMockClient) ListSinceBlock(blockHash *chainhash.Hash) (*btcjson.ListSinceBlockResult, error) {
	panic("not implemented: ListSinceBlock")
}

func (b BitcoindRpcMockClient) ListSinceBlockAsync(blockHash *chainhash.Hash) rpcclient.FutureListSinceBlockResult {
	panic("not implemented: ListSinceBlockAsync")
}

func (b BitcoindRpcMockClient) ListSinceBlockMinConf(blockHash *chainhash.Hash, minConfirms int) (*btcjson.ListSinceBlockResult, error) {
	panic("not implemented: ListSinceBlockMinConf")
}

func (b BitcoindRpcMockClient) ListSinceBlockMinConfAsync(blockHash *chainhash.Hash, minConfirms int) rpcclient.FutureListSinceBlockResult {
	panic("not implemented: ListSinceBlockMinConfAsync")
}

func (b BitcoindRpcMockClient) ListTransactions(account string) ([]btcjson.ListTransactionsResult, error) {
	panic("not implemented: ListTransactions")
}

func (b BitcoindRpcMockClient) ListTransactionsAsync(account string) rpcclient.FutureListTransactionsResult {
	panic("not implemented: ListTransactionsAsync")
}

func (b BitcoindRpcMockClient) ListTransactionsCount(account string, count int) ([]btcjson.ListTransactionsResult, error) {
	panic("not implemented: ListTransactionsCount")
}

func (b BitcoindRpcMockClient) ListTransactionsCountAsync(account string, count int) rpcclient.FutureListTransactionsResult {
	panic("not implemented: ListTransactionsCountAsync")
}

func (b BitcoindRpcMockClient) ListTransactionsCountFrom(account string, count, from int) ([]btcjson.ListTransactionsResult, error) {
	panic("not implemented: ListTransactionsCountFrom")
}

func (b BitcoindRpcMockClient) ListTransactionsCountFromAsync(account string, count, from int) rpcclient.FutureListTransactionsResult {
	panic("not implemented: ListTransactionsCountFromAsync")
}

func (b BitcoindRpcMockClient) ListUnspent() ([]btcjson.ListUnspentResult, error) {
	panic("not implemented: ListUnspent")
}

func (b BitcoindRpcMockClient) ListUnspentAsync() rpcclient.FutureListUnspentResult {
	panic("not implemented: ListUnspentAsync")
}

func (b BitcoindRpcMockClient) ListUnspentMin(minConf int) ([]btcjson.ListUnspentResult, error) {
	panic("not implemented: ListUnspentMin")
}

func (b BitcoindRpcMockClient) ListUnspentMinAsync(minConf int) rpcclient.FutureListUnspentResult {
	panic("not implemented: ListUnspentMinAsync")
}

func (b BitcoindRpcMockClient) ListUnspentMinMax(minConf, maxConf int) ([]btcjson.ListUnspentResult, error) {
	panic("not implemented: ListUnspentMinMax")
}

func (b BitcoindRpcMockClient) ListUnspentMinMaxAddresses(minConf, maxConf int, addrs []btcutil.Address) ([]btcjson.ListUnspentResult, error) {
	panic("not implemented: ListUnspentMinMaxAddresses")
}

func (b BitcoindRpcMockClient) ListUnspentMinMaxAddressesAsync(minConf, maxConf int, addrs []btcutil.Address) rpcclient.FutureListUnspentResult {
	panic("not implemented: ListUnspentMinMaxAddressesAsync")
}

func (b BitcoindRpcMockClient) ListUnspentMinMaxAsync(minConf, maxConf int) rpcclient.FutureListUnspentResult {
	panic("not implemented: ListUnspentMinMaxAsync")
}

func (b BitcoindRpcMockClient) LoadTxFilter(reload bool, addresses []btcutil.Address, outPoints []wire.OutPoint) error {
	panic("not implemented: LoadTxFilter")
}

func (b BitcoindRpcMockClient) LoadTxFilterAsync(reload bool, addresses []btcutil.Address, outPoints []wire.OutPoint) rpcclient.FutureLoadTxFilterResult {
	panic("not implemented: LoadTxFilterAsync")
}

func (b BitcoindRpcMockClient) LockUnspent(unlock bool, ops []*wire.OutPoint) error {
	panic("not implemented: LockUnspent")
}

func (b BitcoindRpcMockClient) LockUnspentAsync(unlock bool, ops []*wire.OutPoint) rpcclient.FutureLockUnspentResult {
	panic("not implemented: LockUnspentAsync")
}

func (b BitcoindRpcMockClient) Move(fromAccount, toAccount string, amount btcutil.Amount) (bool, error) {
	panic("not implemented: Move")
}

func (b BitcoindRpcMockClient) MoveAsync(fromAccount, toAccount string, amount btcutil.Amount) rpcclient.FutureMoveResult {
	panic("not implemented: MoveAsync")
}

func (b BitcoindRpcMockClient) MoveComment(fromAccount, toAccount string, amount btcutil.Amount, minConf int, comment string) (bool, error) {
	panic("not implemented: MoveComment")
}

func (b BitcoindRpcMockClient) MoveCommentAsync(fromAccount, toAccount string, amount btcutil.Amount, minConfirms int, comment string) rpcclient.FutureMoveResult {
	panic("not implemented: MoveCommentAsync")
}

func (b BitcoindRpcMockClient) MoveMinConf(fromAccount, toAccount string, amount btcutil.Amount, minConf int) (bool, error) {
	panic("not implemented: MoveMinConf")
}

func (b BitcoindRpcMockClient) MoveMinConfAsync(fromAccount, toAccount string, amount btcutil.Amount, minConfirms int) rpcclient.FutureMoveResult {
	panic("not implemented: MoveMinConfAsync")
}

func (b BitcoindRpcMockClient) NextID() uint64 {
	panic("not implemented: NextID")
}

func (b BitcoindRpcMockClient) Node(command btcjson.NodeSubCmd, host string, connectSubCmd *string) error {
	panic("not implemented: Node")
}

func (b BitcoindRpcMockClient) NodeAsync(command btcjson.NodeSubCmd, host string, connectSubCmd *string) rpcclient.FutureNodeResult {
	panic("not implemented: NodeAsync")
}

func (b BitcoindRpcMockClient) NotifyBlocks() error {
	panic("not implemented: NotifyBlocks")
}

func (b BitcoindRpcMockClient) NotifyBlocksAsync() rpcclient.FutureNotifyBlocksResult {
	panic("not implemented: NotifyBlocksAsync")
}

func (b BitcoindRpcMockClient) NotifyNewTransactions(verbose bool) error {
	panic("not implemented: NotifyNewTransactions")
}

func (b BitcoindRpcMockClient) NotifyNewTransactionsAsync(verbose bool) rpcclient.FutureNotifyNewTransactionsResult {
	panic("not implemented: NotifyNewTransactionsAsync")
}

func (b BitcoindRpcMockClient) NotifyReceived(addresses []btcutil.Address) error {
	panic("not implemented: NotifyReceived")
}

func (b BitcoindRpcMockClient) NotifyReceivedAsync(addresses []btcutil.Address) rpcclient.FutureNotifyReceivedResult {
	panic("not implemented: NotifyReceivedAsync")
}

func (b BitcoindRpcMockClient) NotifySpent(outpoints []*wire.OutPoint) error {
	panic("not implemented: NotifySpent")
}

func (b BitcoindRpcMockClient) NotifySpentAsync(outpoints []*wire.OutPoint) rpcclient.FutureNotifySpentResult {
	panic("not implemented: NotifySpentAsync")
}

func (b BitcoindRpcMockClient) Ping() error {
	panic("not implemented: Ping")
}

func (b BitcoindRpcMockClient) PingAsync() rpcclient.FuturePingResult {
	panic("not implemented: PingAsync")
}

func (b BitcoindRpcMockClient) RawRequest(method string, params []json.RawMessage) (json.RawMessage, error) {
	panic("not implemented: RawRequest")
}

func (b BitcoindRpcMockClient) RawRequestAsync(method string, params []json.RawMessage) rpcclient.FutureRawResult {
	panic("not implemented: RawRequestAsync")
}

func (b BitcoindRpcMockClient) RenameAccount(oldAccount, newAccount string) error {
	panic("not implemented: RenameAccount")
}

func (b BitcoindRpcMockClient) RenameAccountAsync(oldAccount, newAccount string) rpcclient.FutureRenameAccountResult {
	panic("not implemented: RenameAccountAsync")
}

func (b BitcoindRpcMockClient) Rescan(startBlock *chainhash.Hash, addresses []btcutil.Address, outpoints []*wire.OutPoint) error {
	panic("not implemented: Rescan")
}

func (b BitcoindRpcMockClient) RescanAsync(startBlock *chainhash.Hash, addresses []btcutil.Address, outpoints []*wire.OutPoint) rpcclient.FutureRescanResult {
	panic("not implemented: RescanAsync")
}

func (b BitcoindRpcMockClient) RescanBlocks(blockHashes []chainhash.Hash) ([]btcjson.RescannedBlock, error) {
	panic("not implemented: RescanBlocks")
}

func (b BitcoindRpcMockClient) RescanBlocksAsync(blockHashes []chainhash.Hash) rpcclient.FutureRescanBlocksResult {
	panic("not implemented: RescanBlocksAsync")
}

func (b BitcoindRpcMockClient) RescanEndBlockAsync(startBlock *chainhash.Hash, addresses []btcutil.Address, outpoints []*wire.OutPoint, endBlock *chainhash.Hash) rpcclient.FutureRescanResult {
	panic("not implemented: RescanEndBlockAsync")
}

func (b BitcoindRpcMockClient) RescanEndHeight(startBlock *chainhash.Hash, addresses []btcutil.Address, outpoints []*wire.OutPoint, endBlock *chainhash.Hash) error {
	panic("not implemented: RescanEndHeight")
}

func (b BitcoindRpcMockClient) SearchRawTransactions(address btcutil.Address, skip, count int, reverse bool, filterAddrs []string) ([]*wire.MsgTx, error) {
	panic("not implemented: SearchRawTransactions")
}

func (b BitcoindRpcMockClient) SearchRawTransactionsAsync(address btcutil.Address, skip, count int, reverse bool, filterAddrs []string) rpcclient.FutureSearchRawTransactionsResult {
	panic("not implemented: SearchRawTransactionsAsync")
}

func (b BitcoindRpcMockClient) SearchRawTransactionsVerbose(address btcutil.Address, skip, count int, includePrevOut, reverse bool, filterAddrs []string) ([]*btcjson.SearchRawTransactionsResult, error) {
	panic("not implemented: SearchRawTransactionsVerbose")
}

func (b BitcoindRpcMockClient) SearchRawTransactionsVerboseAsync(address btcutil.Address, skip, count int, includePrevOut, reverse bool, filterAddrs *[]string) rpcclient.FutureSearchRawTransactionsVerboseResult {
	panic("not implemented: SearchRawTransactionsVerboseAsync")
}

func (b BitcoindRpcMockClient) SendFrom(fromAccount string, toAddress btcutil.Address, amount btcutil.Amount) (*chainhash.Hash, error) {
	panic("not implemented: SendFrom")
}

func (b BitcoindRpcMockClient) SendFromAsync(fromAccount string, toAddress btcutil.Address, amount btcutil.Amount) rpcclient.FutureSendFromResult {
	panic("not implemented: SendFromAsync")
}

func (b BitcoindRpcMockClient) SendFromComment(fromAccount string, toAddress btcutil.Address, amount btcutil.Amount, minConfirms int, comment, commentTo string) (*chainhash.Hash, error) {
	panic("not implemented: SendFromComment")
}

func (b BitcoindRpcMockClient) SendFromCommentAsync(fromAccount string, toAddress btcutil.Address, amount btcutil.Amount, minConfirms int, comment, commentTo string) rpcclient.FutureSendFromResult {
	panic("not implemented: SendFromCommentAsync")
}

func (b BitcoindRpcMockClient) SendFromMinConf(fromAccount string, toAddress btcutil.Address, amount btcutil.Amount, minConfirms int) (*chainhash.Hash, error) {
	panic("not implemented: SendFromMinConf")
}

func (b BitcoindRpcMockClient) SendFromMinConfAsync(fromAccount string, toAddress btcutil.Address, amount btcutil.Amount, minConfirms int) rpcclient.FutureSendFromResult {
	panic("not implemented: SendFromMinConfAsync")
}

func (b BitcoindRpcMockClient) SendMany(fromAccount string, amounts map[btcutil.Address]btcutil.Amount) (*chainhash.Hash, error) {
	panic("not implemented: SendMany")
}

func (b BitcoindRpcMockClient) SendManyAsync(fromAccount string, amounts map[btcutil.Address]btcutil.Amount) rpcclient.FutureSendManyResult {
	panic("not implemented: SendManyAsync")
}

func (b BitcoindRpcMockClient) SendManyComment(fromAccount string, amounts map[btcutil.Address]btcutil.Amount, minConfirms int, comment string) (*chainhash.Hash, error) {
	panic("not implemented: SendManyComment")
}

func (b BitcoindRpcMockClient) SendManyCommentAsync(fromAccount string, amounts map[btcutil.Address]btcutil.Amount, minConfirms int, comment string) rpcclient.FutureSendManyResult {
	panic("not implemented: SendManyCommentAsync")
}

func (b BitcoindRpcMockClient) SendManyMinConf(fromAccount string, amounts map[btcutil.Address]btcutil.Amount, minConfirms int) (*chainhash.Hash, error) {
	panic("not implemented: SendManyMinConf")
}

func (b BitcoindRpcMockClient) SendManyMinConfAsync(fromAccount string, amounts map[btcutil.Address]btcutil.Amount, minConfirms int) rpcclient.FutureSendManyResult {
	panic("not implemented: SendManyMinConfAsync")
}

func (b BitcoindRpcMockClient) SendRawTransaction(tx *wire.MsgTx, allowHighFees bool) (*chainhash.Hash, error) {
	panic("not implemented: SendRawTransaction")
}

func (b BitcoindRpcMockClient) SendRawTransactionAsync(tx *wire.MsgTx, allowHighFees bool) rpcclient.FutureSendRawTransactionResult {
	panic("not implemented: SendRawTransactionAsync")
}

func (b BitcoindRpcMockClient) SendToAddress(address btcutil.Address, amount btcutil.Amount) (*chainhash.Hash, error) {
	panic("not implemented: SendToAddress")
}

func (b BitcoindRpcMockClient) SendToAddressAsync(address btcutil.Address, amount btcutil.Amount) rpcclient.FutureSendToAddressResult {
	panic("not implemented: SendToAddressAsync")
}

func (b BitcoindRpcMockClient) SendToAddressComment(address btcutil.Address, amount btcutil.Amount, comment, commentTo string) (*chainhash.Hash, error) {
	panic("not implemented: SendToAddressComment")
}

func (b BitcoindRpcMockClient) SendToAddressCommentAsync(address btcutil.Address, amount btcutil.Amount, comment, commentTo string) rpcclient.FutureSendToAddressResult {
	panic("not implemented: SendToAddressCommentAsync")
}

func (b BitcoindRpcMockClient) Session() (*btcjson.SessionResult, error) {
	panic("not implemented: Session")
}

func (b BitcoindRpcMockClient) SessionAsync() rpcclient.FutureSessionResult {
	panic("not implemented: SessionAsync")
}

func (b BitcoindRpcMockClient) SetAccount(address btcutil.Address, account string) error {
	panic("not implemented: SetAccount")
}

func (b BitcoindRpcMockClient) SetAccountAsync(address btcutil.Address, account string) rpcclient.FutureSetAccountResult {
	panic("not implemented: SetAccountAsync")
}

func (b BitcoindRpcMockClient) SetGenerate(enable bool, numCPUs int) error {
	panic("not implemented: SetGenerate")
}

func (b BitcoindRpcMockClient) SetGenerateAsync(enable bool, numCPUs int) rpcclient.FutureSetGenerateResult {
	panic("not implemented: SetGenerateAsync")
}

func (b BitcoindRpcMockClient) SetTxFee(fee btcutil.Amount) error {
	panic("not implemented: SetTxFee")
}

func (b BitcoindRpcMockClient) SetTxFeeAsync(fee btcutil.Amount) rpcclient.FutureSetTxFeeResult {
	panic("not implemented: SetTxFeeAsync")
}

func (b BitcoindRpcMockClient) Shutdown() {
	panic("not implemented: Shutdown")
}

func (b BitcoindRpcMockClient) SignMessage(address btcutil.Address, message string) (string, error) {
	panic("not implemented: SignMessage")
}

func (b BitcoindRpcMockClient) SignMessageAsync(address btcutil.Address, message string) rpcclient.FutureSignMessageResult {
	panic("not implemented: SignMessageAsync")
}

func (b BitcoindRpcMockClient) SignRawTransaction(tx *wire.MsgTx) (*wire.MsgTx, bool, error) {
	panic("not implemented: SignRawTransaction")
}

func (b BitcoindRpcMockClient) SignRawTransaction2(tx *wire.MsgTx, inputs []btcjson.RawTxInput) (*wire.MsgTx, bool, error) {
	panic("not implemented: SignRawTransaction2")
}

func (b BitcoindRpcMockClient) SignRawTransaction2Async(tx *wire.MsgTx, inputs []btcjson.RawTxInput) rpcclient.FutureSignRawTransactionResult {
	panic("not implemented: SignRawTransaction2Async")
}

func (b BitcoindRpcMockClient) SignRawTransaction3(tx *wire.MsgTx, inputs []btcjson.RawTxInput, privKeysWIF []string) (*wire.MsgTx, bool, error) {
	panic("not implemented: SignRawTransaction3")
}

func (b BitcoindRpcMockClient) SignRawTransaction3Async(tx *wire.MsgTx, inputs []btcjson.RawTxInput, privKeysWIF []string) rpcclient.FutureSignRawTransactionResult {
	panic("not implemented: SignRawTransaction3Async")
}

func (b BitcoindRpcMockClient) SignRawTransaction4(tx *wire.MsgTx, inputs []btcjson.RawTxInput, privKeysWIF []string, hashType rpcclient.SigHashType) (*wire.MsgTx, bool, error) {
	panic("not implemented: SignRawTransaction4")
}

func (b BitcoindRpcMockClient) SignRawTransaction4Async(tx *wire.MsgTx, inputs []btcjson.RawTxInput, privKeysWIF []string, hashType rpcclient.SigHashType) rpcclient.FutureSignRawTransactionResult {
	panic("not implemented: SignRawTransaction4Async")
}

func (b BitcoindRpcMockClient) SignRawTransactionAsync(tx *wire.MsgTx) rpcclient.FutureSignRawTransactionResult {
	panic("not implemented: SignRawTransactionAsync")
}

func (b BitcoindRpcMockClient) SubmitBlock(block *btcutil.Block, options *btcjson.SubmitBlockOptions) error {
	panic("not implemented: SubmitBlock")
}

func (b BitcoindRpcMockClient) SubmitBlockAsync(block *btcutil.Block, options *btcjson.SubmitBlockOptions) rpcclient.FutureSubmitBlockResult {
	panic("not implemented: SubmitBlockAsync")
}

func (b BitcoindRpcMockClient) ValidateAddress(address btcutil.Address) (*btcjson.ValidateAddressWalletResult, error) {
	panic("not implemented: ValidateAddress")
}

func (b BitcoindRpcMockClient) ValidateAddressAsync(address btcutil.Address) rpcclient.FutureValidateAddressResult {
	panic("not implemented: ValidateAddressAsync")
}

func (b BitcoindRpcMockClient) VerifyChain() (bool, error) {
	panic("not implemented: VerifyChain")
}

func (b BitcoindRpcMockClient) VerifyChainAsync() rpcclient.FutureVerifyChainResult {
	panic("not implemented: VerifyChainAsync")
}

func (b BitcoindRpcMockClient) VerifyChainBlocks(checkLevel, numBlocks int32) (bool, error) {
	panic("not implemented: VerifyChainBlocks")
}

func (b BitcoindRpcMockClient) VerifyChainBlocksAsync(checkLevel, numBlocks int32) rpcclient.FutureVerifyChainResult {
	panic("not implemented: VerifyChainBlocksAsync")
}

func (b BitcoindRpcMockClient) VerifyChainLevel(checkLevel int32) (bool, error) {
	panic("not implemented: VerifyChainLevel")
}

func (b BitcoindRpcMockClient) VerifyChainLevelAsync(checkLevel int32) rpcclient.FutureVerifyChainResult {
	panic("not implemented: VerifyChainLevelAsync")
}

func (b BitcoindRpcMockClient) VerifyMessage(address btcutil.Address, signature, message string) (bool, error) {
	panic("not implemented: VerifyMessage")
}

func (b BitcoindRpcMockClient) VerifyMessageAsync(address btcutil.Address, signature, message string) rpcclient.FutureVerifyMessageResult {
	panic("not implemented: VerifyMessageAsync")
}

func (b BitcoindRpcMockClient) Version() (map[string]btcjson.VersionResult, error) {
	panic("not implemented: Version")
}

func (b BitcoindRpcMockClient) VersionAsync() rpcclient.FutureVersionResult {
	panic("not implemented: VersionAsync")
}

func (b BitcoindRpcMockClient) WaitForShutdown() {
	panic("not implemented: WaitForShutdown")
}

func (b BitcoindRpcMockClient) WalletLock() error {
	panic("not implemented: WalletLock")
}

func (b BitcoindRpcMockClient) WalletLockAsync() rpcclient.FutureWalletLockResult {
	panic("not implemented: WalletLockAsync")
}

func (b BitcoindRpcMockClient) WalletPassphrase(passphrase string, timeoutSecs int64) error {
	panic("not implemented: WalletPassphrase")
}

func (b BitcoindRpcMockClient) WalletPassphraseChange(old, new string) error {
	panic("not implemented: WalletPassphraseChange")
}

func (b BitcoindRpcMockClient) WalletPassphraseChangeAsync(old, new string) rpcclient.FutureWalletPassphraseChangeResult {
	panic("not implemented: WalletPassphraseChangeAsync")
}
