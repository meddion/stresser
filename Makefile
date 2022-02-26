build:
	go build -o antiprop -v .

docker_build:
	docker build -t antiprop .

docker_run:
	docker run --rm -it --name=antiprop lovefromukraine/antiprop --refresh=20 \
	--dnsres=true --errcount=5 --onlyproxy=false --bots 10 --checkproxy=true \
	--sites https://raw.githubusercontent.com/meddion/stresser/sources/targets.json \
	--proxy https://raw.githubusercontent.com/meddion/stresser/sources/proxy.json

docker_push:
	docker build -t antiprop .
	docker tag antiprop:latest lovefromukraine/antiprop:latest
	docker push lovefromukraine/antiprop:latest

test_curl_proxy:
	curl -v https://gebank.ru/ -x  95.164.235.38:6094 -U spiznxfg:r6daod3mfgkz

test_curl_proxy:
	curl -v https://google.com/ -x 118.97.180.131:30793

test_nc:
	while :; do nc -l -p 8080 | tee  output.log; sleep 1; done

run:
	./antiprop --refresh=20 --dnsres=true --errcount=5 --onlyproxy=false --bots 2 --checkproxy=false \
	--sites https://raw.githubusercontent.com/meddion/stresser/sources/targets.json \
	--proxy https://raw.githubusercontent.com/meddion/stresser/sources/proxy.json

run_file:
	./antiprop --checkproxy=false --onlyproxy --api 2 --bots 3 \
		--file ./examples/api_v2_src_resp1.json
