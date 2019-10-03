# fish programmable completion for bitcoin-cli
# copy to $HOME/.config/fish/completions and restart your shell session

function __fish_has_not_seen_opt --argument opt --description "Checks if an option has been given"
    for i in (commandline -opc)
        switch $i
            case "$opt*"
                return 1
        end
    end
    return 0
end

function __fish_has_not_seen_command --description "Checks if any command has been given"
    set seen_commands 0
    for i in (commandline -opc)
        switch $i
            # option has been given, continue
            case "-*"
            case "*"
                set seen_commands (math $seen_commands + 1)
        end
    end
    # if we're in the middle of typing a command
    # seen_commands will be 1, but we would still
    # like to get suggestions for our current
    # command
    test $seen_commands -lt 2
end


# when an empty list of completions is provided we still get
# '=' appended to the end of our option
function __fish_empty_complete --description "Provide an empty list of completions"
    return ""
end

function __fish_bitcoind_complete_opt --argument cmd desc complete_func
    set full_cmd '-'$cmd

    # if a completion function was provided
    # we append '=' to our option
    if test -n "$complete_func"
        set full_cmd $full_cmd'='
    end

    set has_not_seen_cmd_or_opt "__fish_has_not_seen_opt $full_cmd; and __fish_has_not_seen_command"

    # only suggest this option if it hasn't been seen before
    complete -c bitcoin-cli -a $full_cmd -d $desc -n $has_not_seen_cmd_or_opt

    # if a completion function is provided, we provide more completions
    if test -n "$complete_func"
        # only display completions if the current entered line
        # matches the full command, i.e. we're past the '='
        set show_more_cond "string match -q -- \"$full_cmd\" (commandline -ct)"
        complete -c bitcoin-cli -a $full_cmd"($fish_complete_func)" -n $show_more_cond -d $desc
    end
end

__fish_bitcoind_complete_opt conf "Specify configuration path" __fish_complete_path
__fish_bitcoind_complete_opt datadir "Specify data directory" __fish_complete_directories
__fish_bitcoind_complete_opt getinfo "Get general information"
__fish_bitcoind_complete_opt named "Pass named instead of positional arguments"
__fish_bitcoind_complete_opt rpcclienttimeout "Timeout in seconds during HTTP requests (0 for no timeout)" __fish_empty_complete
__fish_bitcoind_complete_opt pcconnect "Send commands to node running on <ip>" __fish_empty_complete
__fish_bitcoind_complete_opt rpccookiefile "Location of the auth cookie" __fish_complete_path
__fish_bitcoind_complete_opt rpcport "Connect to JSON-RPC on <port>" __fish_empty_complete
__fish_bitcoind_complete_opt rpcuser "Username for JSON-RPC connections" __fish_empty_complete
__fish_bitcoind_complete_opt rpcwait "Wait for the RPC server to start"
__fish_bitcoind_complete_opt rpcwallet "Send RPC for non-default wallet" __fish_empty_complete
__fish_bitcoind_complete_opt stdin "Read extra arguments from standard input"
__fish_bitcoind_complete_opt stdinrpcpass "Read RPC password from standard input as a single line"
__fish_bitcoind_complete_opt version "Print version and exit"
__fish_bitcoind_complete_opt testnet "Use the test chain"
complete -c bitcoin-cli -s "?" -f -d "Display help" -n __fish_has_not_seen_command


set commands getbestblockhash getblock getblockchaininfo getblockcount getblockhash getblockheader getblockstats getchaintips getchaintxstats getdifficulty getmempoolancestors getmempooldescendants getmempoolentry getmempoolinfo getrawmempool gettxout gettxoutproof gettxoutsetinfo preciousblock pruneblockchain savemempool scantxoutset verifychain verifytxoutproof getmemoryinfo getrpcinfo help logging stop uptime generate generatetoaddress getblocktemplate getmininginfo getnetworkhashps prioritisetransaction submitblock submitheader addnode clearbanned disconnectnode getaddednodeinfo getconnectioncount getnettotals getnetworkinfo getnodeaddresses getpeerinfo listbanned ping setban setnetworkactive analyzepsbt combinepsbt combinerawtransaction converttopsbt createpsbt createrawtransaction decodepsbt decoderawtransaction decodescript finalizepsbt fundrawtransaction getrawtransaction joinpsbts sendrawtransaction signrawtransactionwithkey testmempoolaccept utxoupdatepsbt createmultisig deriveaddresses estimatesmartfee getdescriptorinfo signmessagewithprivkey validateaddress verifymessage abandontransaction abortrescan addmultisigaddress backupwallet bumpfee createwallet dumpprivkey dumpwallet encryptwallet getaddressesbylabel getaddressinfo getbalance getnewaddress getrawchangeaddress getreceivedbyaddress getreceivedbylabel gettransaction getunconfirmedbalance getwalletinfo importaddress importmulti importprivkey importprunedfunds importpubkey importwallet keypoolrefill listaddressgroupings listlabels listlockunspent listreceivedbyaddress listreceivedbylabel listsinceblock listtransactions listunspent listwalletdir listwallets loadwallet lockunspent removeprunedfunds rescanblockchain sendmany sendtoaddress sethdseed setlabel settxfee signmessage signrawtransactionwithwallet unloadwallet walletcreatefundedpsbt walletlock walletpassphrase walletpassphrasechange walletprocesspsbt getzmqnotifications

complete -c bitcoin-cli -x -a "$commands" -n __fish_has_not_seen_command
