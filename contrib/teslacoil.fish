# https://medium.com/@fabioantunes/a-guide-for-fish-shell-completions-485ac04ac63c
# Fish shell completions for `teslacoil`

set teslacoil_subcommands "db serve help"
set teslacoil_db_subcommands "down up status newmigration drop dummy"

complete -c teslacoil -f -a $teslacoil_subcommands -n "not __fish_seen_subcommand_from $teslacoil_subcommands"
complete -c teslacoil -f -a $teslacoil_db_subcommands -n "__fish_seen_subcommand_from db"
complete -c teslacoil -l lnddir -r
complete -c teslacoil -l tlscertpath -r
complete -c teslacoil -l macaroonpath -r
complete -c teslacoil -l network -r -x -a "mainnet testnet regtest"
complete -c teslacoil -l lndrpcserver -x
complete -c teslacoil -l loglevel -r -x -a "trace debug info warn error fatal critical"
complete -c teslacoil -l help -s h -f
complete -c teslacoil -l version -s v -f