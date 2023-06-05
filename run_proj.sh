cd ./server
go run . &
cd ../client
sleep 3
go run .
cd ..
