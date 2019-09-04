# https://medium.com/@fabioantunes/a-guide-for-fish-shell-completions-485ac04ac63c
# Fish shell completions for `lpp`

set lpp_subcommands "db serve help"
set lpp_db_subcommands "down up status newmigration drop dummy"

complete -c lpp -f -a $lpp_subcommands -n "not __fish_seen_subcommand_from $lpp_subcommands"
complete -c lpp -f -a $lpp_db_subcommands -n "__fish_seen_subcommand_from db"
complete -c lpp -l lnddir -r
complete -c lpp -l tlscertpath -r
complete -c lpp -l macaroonpath -r
complete -c lpp -l network -r -x -a "mainnet testnet regtest simnet"
complete -c lpp -l lndrpcserver -x
complete -c lpp -l debuglevel -r -x -a "trace debug info warn error critical off"
complete -c lpp -l help -s h -f
complete -c lpp -l version -s v -f