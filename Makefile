sweeper: main.go minefield.go player.go server.go
	go build

sweeper-openbsd: main.go minefield.go player.go server.go
	GOOS=openbsd go build

upload: sweeper-openbsd
	rsync -Phr sweeper static unobtanium.de:sweeper/