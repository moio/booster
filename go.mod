module github.com/moio/booster

go 1.16

require (
	github.com/itchio/headway v0.0.0-20200301160421-e15721f23905
	github.com/itchio/lake v0.0.0-20200305150023-cc4284ec2b2a
	github.com/itchio/savior v0.0.0-20200303195615-7cac7998294c
	github.com/itchio/wharf v0.0.0-20200618133039-e0beba741312
	github.com/pkg/errors v0.9.1
	github.com/rs/zerolog v1.23.0
	github.com/urfave/cli v1.22.5
)

replace github.com/itchio/lake => github.com/moio/lake v0.0.0-20210618151745-df1660885716

replace github.com/itchio/wharf => github.com/moio/wharf v0.0.0-20210708091113-5b942a14d1d5

replace github.com/itchio/savior => github.com/moio/savior v0.0.0-20210708090523-e76614c56ce5
