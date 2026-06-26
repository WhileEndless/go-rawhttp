# rawhttp — curl benzeri CLI

`go-rawhttp` kütüphanesini backend olarak kullanan, **curl uyumlu** bir komut
satırı istemcisi. Standart curl ergonomisinin yanında kütüphanenin ham-HTTP
süpergüçlerini (ham istek gönderme, SNI kontrolü, doğrudan IP'ye bağlanma, tüm
proxy türleri, açık HTTP/2, mTLS, bağlantı yeniden kullanımı, zengin TLS/timing
metadata) ve **çok bağlantılı (IDM tarzı) indirme yöneticisini** sunar.

## Derleme & kurulum

CLI artık ayrı bir Go modülüdür (renklendirme/beautify bağımlılıkları yalnızca
burada; `go-rawhttp` kütüphanesi minimal kalır). Bu yüzden kökten
`go build ./cmd/rawhttp` **çalışmaz**. Repo kökünden Makefile'ı kullanın:

```sh
make build                 # ./rawhttp derler
make install               # ~/.local/bin'e kurar (root gerekmez, mac+linux)
make install BINDIR=~/bin  # farklı bir dizine kur
make uninstall             # kaldır
```

Makefile yoksa elle: `cd cmd/rawhttp && go build -o rawhttp .`

> `cmd/rawhttp/go.mod` içinde kütüphaneyi yereldeki kopyaya bağlayan bir
> `replace` direktifi vardır; bu yüzden `go install ...@latest` desteklenmez —
> kaynaktan derleyin.

## Hızlı başlangıç

```sh
rawhttp https://example.com                 # GET, gövdeyi stdout'a yaz
rawhttp -i https://example.com              # yanıt başlıklarını da dahil et
rawhttp -I https://example.com              # sadece başlıklar (HEAD)
rawhttp -v https://example.com              # ayrıntılı iz (>, <, * satırları)
rawhttp -d 'a=1&b=2' https://httpbin.org/post   # POST form verisi
rawhttp -G -d 'q=x' https://httpbin.org/get     # veriyi query'e taşı
rawhttp -F 'f=@dosya.txt' https://httpbin.org/post  # multipart yükleme
rawhttp -L https://httpbin.org/redirect/3   # yönlendirmeleri takip et
rawhttp -o out.html https://example.com     # dosyaya yaz
```

## Bayraklar

### Standart (curl uyumlu)
`-X/--request`, `-H/--header` (tekrarlanabilir), `-d/--data`, `--data-binary`,
`--data-raw`, `--data-hex`, `--data-base64`, `-F/--form`, `-A/--user-agent`, `-e/--referer`, `-b/--cookie`,
`-u/--user`, `-G/--get`, `-I/--head`, `-L/--location`, `--max-redirs`,
`-o/--output`, `-O/--remote-name`, `-s/--silent`, `-v/--verbose`,
`-i/--include`, `-k/--insecure`, `--connect-timeout`, `-m/--max-time`,
`-x/--proxy`, `--proxy-user`, `-w/--write-out`, `--http1.1`, `--http2`
(h2 dene, olmazsa 1.1'e düş), `--http2-prior-knowledge` (katı h2),
`--resolve`, `--cert`, `--key`, `--cacert`, `-V/--version`,
`--compressed` (varsayılan açık) / `--no-compressed`, `--data-urlencode`,
`-f/--fail`, `-r/--range`, `-g/--globoff`, `-N/--no-buffer`, `--no-keepalive`,
`--path-as-is`.

> **Not:** rawhttp varsayılan olarak `Accept-Encoding: gzip, deflate, br` gönderir
> ve yanıtı otomatik açar (tarayıcı gibi). Bazı sunucular yalnızca sıkıştırma
> isteyen istemcilere gerçek sayfayı döner; bu yüzden bu varsayılan "gerçek"
> yanıtı almanızı sağlar. Ham tel için `--no-compressed` kullanın.

### Yapılandırılmış çıktı & istek gövdesi
- `--json` / `--xml` — tüm işlemi (istek, yanıt, bağlantı, TLS, proxy, timing
  istatistikleri ve hata) tek bir belge olarak ver; başarıda da hatada da üretilir,
  stdout'a veya `-o dosya`'ya. Gövdeler `--max-render-size` ile sınırlanır.
- `--html <dosya>` — aynı işlemi **stilize, syntax-highlight'lı bir HTML raporu**
  olarak dosyaya yaz: istek/yanıt/başlıklar/gövdeler + bağlantı/TLS/timing
  istatistikleri, ham istek ve yanıt için **kopyalama butonları**. Sadece dosyaya
  yazılır (CLI'a basılmaz).
- `--theme dark|light` — hem terminal hem `--html` için tema-uygun highlight
  stili (dark → monokai, light → github). `--style` ile açık chroma stili verilebilir.

### Beautify kapsamı
JSON/XML/HTML tam beautify; **HTML içindeki gömülü `<script>` JS ve `<style>` CSS
de** (ve bağımsız JS/CSS gövdeler) esbuild ile yeniden biçimlenir (minified kod
açılır) ve etiket altında girintilenir. `<script type="...json">` JSON olarak
biçimlenir.
- **Content-Length**: elle verirseniz **aynen gönderilir** (curl gibi, asla
  ezilmez); vermezseniz otomatik hesaplanır. `--no-content-length` ile otomatik
  eklemeyi kapatabilirsiniz. Elle verdiğiniz değer gövde boyutuyla uyuşmazsa
  uyarı verilir (çoğunlukla gövdenin komut satırında NUL'da kesilmesi).
- İkili (binary) gövdeler komut satırında **NUL baytında kesilir** (shell sınırı):
  `$'...\x00...'` argümanı, rawhttp'ye ulaşmadan önce ilk NUL'da biter ("Content-Length
  does not match the request body size" uyarısının sebebi budur). Tam ikili gövde için:
  - `--data-binary @dosya` — gövdeyi dosyadan oku (NUL içerebilir), ya da
  - `--data-hex <hex>` / `--data-base64 <b64>` — gövdeyi hex/base64 olarak ASCII güvenli
    geçir; rawhttp ham bayta çevirir. İkisi de `@dosya` (ve `@-` stdin), boşluk/satır
    sonu toleransı, hex'te `0x` öneki ve std/url base64 (padding'li/padding'siz) destekler.
    Çözülen uzunluk Content-Length'i otomatik besler.

### Kütüphane süpergüçleri
- `--sni <ad>` / `--disable-sni` — TLS SNI'yi değiştir ya da tamamen kapat.
- `--connect-ip <ip>` / `--connect-to` / `--resolve host:port:ip` — DNS'i atlayıp
  doğrudan bir IP'ye bağlan.
- `--raw-request <dosya>` — elle hazırlanmış ham HTTP isteğini (bozuk/yinelenen
  başlıklar dahil) **olduğu gibi** gönder. `-` ile stdin'den okur.
- `--reuse` — keep-alive bağlantı havuzunu etkinleştir.
- `--tls-min` / `--tls-max` — TLS sürüm aralığını belirle (1.0–1.3).
- `--timings` — DNS/TCP/TLS/TTFB/Total kırılımını stderr'e yaz.

### İndirme yöneticisi (çok bağlantılı, IDM tarzı)
- `--download` — segmentli indirme modunu ve ilerleme çubuğunu aç.
- `-j, --parallel <N>` — paralel bağlantı sayısı (>1 ise `--download` ima edilir).
- `--chunk-size <boyut>` — segment boyutu (`512K`, `4M`, `1G` veya bayt).
- `--no-progress` — ilerleme çubuğunu kapat.

```sh
# 8 bağlantı, 1MB segmentlerle indir; ilerleme çubuğu, hız ve ETA göster
rawhttp -j 8 --chunk-size 1M -o film.bin https://cdn.example.com/film.bin
```

Sunucu `Range` desteklemiyorsa otomatik olarak tek bağlantılı indirmeye düşer.
Segmentler dosyanın doğru ofsetlerine `WriteAt` ile paralel yazılır.

## Renklendirme & beautify (varsayılan açık)

Çıktı bir **terminale** giderken yanıt renklendirilir ve gövde içerik türüne göre
beautify edilir (Burp "pretty" görünümü gibi). Çıktı bir **pipe veya dosyaya**
giderse (örn. `| jq`, `> file`, `-o`) ham bayt yazılır — pipeline'lar bozulmaz.

- **Header'lar**: status (2xx yeşil / 3xx camgöbeği / 4xx-5xx kırmızı), header
  adı/değeri, `Set-Cookie`/`Cookie` çiftleri, `Authorization`, `Location`, ve
  istek satırındaki **query parametreleri** renklenir.
- **Gövde**: JSON / XML / HTML beautify + highlight; JS / CSS highlight (HTML
  içindeki gömülü `<script>`/`<style>` kendi diline göre); form (`a=1&b=2`) renklenir.
- **Sıkıştırma**: `Content-Encoding: gzip/deflate/br` otomatik açılır.
- **Binary/görsel**: ham basılmaz; tür/boyut (görselse `WxH`) özetlenir.

Bayraklar:
- `--color` / `--no-color` — renklendirmeyi zorla / kapat (varsayılan: TTY'de otomatik).
  `NO_COLOR` ve `FORCE_COLOR` ortam değişkenleri de geçerlidir.
- `--beautify` / `--no-beautify` — beautify'ı zorla / kapat (varsayılan: TTY'de otomatik).
- `--style <ad>` — syntax-highlight teması (chroma stili; varsayılan `monokai`).
- `--print-binary` — binary/görsel gövdeyi özetlemek yerine ham bas.
- `--max-render-size <bayt>` — beautify/highlight üst sınırı (varsayılan 5 MiB; üstü ham).

> Not: Saf Go'da kararlı bir JS/CSS yeniden-girintileyici olmadığından **minified
> JS/CSS yeniden girintilenmez**, yalnızca highlight edilir. JSON/XML/HTML tam beautify.
> Güvenlik: terminale giden metindeki kontrol/ESC karakterleri temizlenir
> (escape-injection koruması); büyük girdiler sınırlanır (ReDoS/decompression-bomb).

## `-w` değişkenleri

`%{http_code}`, `%{http_version}`, `%{scheme}`, `%{content_type}`,
`%{num_redirects}`, `%{num_connects}`, `%{size_download}`, `%{remote_ip}`,
`%{remote_port}`, `%{local_ip}`, `%{local_port}`, `%{ssl_verify_result}`,
`%{time_namelookup}`, `%{time_connect}`, `%{time_appconnect}`,
`%{time_pretransfer}`, `%{time_starttransfer}`, `%{time_total}`.

```sh
rawhttp -s -o /dev/null \
  -w '%{http_code} %{time_total}s %{remote_ip}\n' https://example.com
```

## Çıkış kodları

curl ile uyumlu: `0` başarı, `3` URL hatası, `6` DNS, `7` bağlantı,
`28` zaman aşımı, `47` çok fazla yönlendirme, `60` TLS sertifika hatası.
