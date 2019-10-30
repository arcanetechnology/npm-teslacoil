# https://medium.com/@fabioantunes/a-guide-for-fish-shell-completions-485ac04ac63c
# Fish shell completions for `tlc`

set tlc_subcommands "db serve help"
set tlc_db_subcommands "down up status newmigration drop dummy"

complete -c tlc -f -a $tlc_subcommands -n "not __fish_seen_subcommand_from $tlc_subcommands"
complete -c tlc -f -a $tlc_db_subcommands -n "__fish_seen_subcommand_from db"
complete -c tlc -l lnddir -r
complete -c tlc -l tlscertpath -r
complete -c tlc -l macaroonpath -r
complete -c tlc -l network -r -x -a "mainnet testnet regtest"
complete -c tlc -l lndrpcserver -x
complete -c tlc -l loglevel -r -x -a "trace debug info warn error fatal critical"
complete -c tlc -l help -s h -f
complete -c tlc -l version -s v -f