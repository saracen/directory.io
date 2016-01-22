package main

import (
	"fmt"
	"log"
	"math/big"
	"net/http"

	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
)

const ResultsPerPage = 128

const PageTemplateHeader = `<!DOCTYPE HTML>
<html>
<head>
	<title>All bitcoin private keys</title>
	<meta charset="utf-8" />
	<style>
		body{font-size: 9pt;}
		a{text-decoration: none}
		a:hover {text-decoration: underline}
		.keys > span:hover { background: #f0f0f0; }
		span:target { background: #ccffcc; }
		td{
			font-family: monospace;
			padding-left: 0.5em;
			padding-right: 0.5em;
			text-align: right
		}
	</style>
</head>
<body>
<h1>Bitcoin private key database</h1>
<h2>Page %s out of %s</h2>
<h3>total: %s</h3>
<a href="/%s">previous</a> | <a href="/%s">next</a>
<table class="keys">
<tr><th>index</th><th>Private Key</th><th>Address</th><th>Compressed Address</th></tr>
`

const PageTemplateFooter = `</table>
<pre style="margin-top: 1em; font-size: 8pt">
It took a lot of computing power to generate this database. Donations welcome: 1Bv8dN7pemC5N3urfMDdAFReibefrBqCaK
</pre>
<a href="/%s">previous</a> | <a href="/%s">next</a>
</body>
</html>`

const KeyTemplate = `<tr><td id="%s">%d</td><td><!-- a href="/warning:understand-how-this-works!/%s">+</a--> <span title="%s">%s </span></td><td><a href="https://blockchain.info/address/%s">%34s</a></td><td><a href="https://blockchain.info/address/%s">%34s</a></td></tr>`

const JsonKeyTemplate=`{"private":"%s", "number":"%s", "compressed":"%s", "uncompressed":"%s"}`
var (
	// Total bitcoins
	total = new(big.Int).SetBytes([]byte{
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFE,
		0xBA, 0xAE, 0xDC, 0xE6, 0xAF, 0x48, 0xA0, 0x3B, 0xBF, 0xD2, 0x5E, 0x8C, 0xD0, 0x36, 0x41, 0x40,
	})

	// One
	one = big.NewInt(1)

	// Total pages
	_pages = new(big.Int).Div(total, big.NewInt(ResultsPerPage))
	pages  = _pages.Add(_pages, one)
)

type Key struct {
	private      string
	number       string
	compressed   string
	uncompressed string
}

func computeSingle(count *big.Int) (key Key){
	var padded [32]byte

	// Copy count value's bytes to padded slice
	copy(padded[32-len(count.Bytes()):], count.Bytes())

	// Get private and public keys
	privKey, public := btcec.PrivKeyFromBytes(btcec.S256(), padded[:])

	// Get compressed and uncompressed addresses for public key
	caddr, _ := btcutil.NewAddressPubKey(public.SerializeCompressed(), &chaincfg.MainNetParams)
	uaddr, _ := btcutil.NewAddressPubKey(public.SerializeUncompressed(), &chaincfg.MainNetParams)

	// Encode addresses
	wif, _ := btcutil.NewWIF(privKey, &chaincfg.MainNetParams, false)
	key.private = wif.String()
	key.number = count.String()
	key.compressed = caddr.EncodeAddress()
	key.uncompressed = uaddr.EncodeAddress()

	return key
}

func compute(count *big.Int) (keys [ResultsPerPage]Key, length int) {
	var i int
	for i = 0; i < ResultsPerPage; i++ {
		// Increment our counter
		count.Add(count, one)

		// Check to make sure we're not out of range
		if count.Cmp(total) > 0 {
			break
		}
		keys [i] = computeSingle (count)
	}
	return keys, i
}

func PageRequest(w http.ResponseWriter, r *http.Request) {
	// Default page is page 1
	if len(r.URL.Path) <= 1 {
		r.URL.Path = "/1"
	}

	// Convert page number to bignum
	page, success := new(big.Int).SetString(r.URL.Path[1:], 0)
	if !success {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Make sure page number cannot be negative or 0
	page.Abs(page)
	if page.Cmp(one) == -1 {
		page.SetInt64(1)
	}

	// Make sure we're not above page count
	if page.Cmp(pages) > 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Get next and previous page numbers
	previous := new(big.Int).Sub(page, one)
	next := new(big.Int).Add(page, one)

	// Calculate our starting key from page number
	start := new(big.Int).Mul(previous, big.NewInt(ResultsPerPage))

	// Send page header
	fmt.Fprintf(w, PageTemplateHeader, page, pages, total, previous, next)

	// Send keys
	keys, length := compute(start)

	for i := 0; i < length; i++ {
		key := keys[i]
		fmt.Fprintf(w, KeyTemplate, key.private, i+1, key.private, key.number, key.private, key.uncompressed, key.uncompressed, key.compressed, key.compressed)
	}

	// Send page footer
	fmt.Fprintf(w, PageTemplateFooter, previous, next)
}

func JsonSingleRequest(w http.ResponseWriter, r *http.Request) {
	// Default page is page 1
	if len(r.URL.Path) <= 1 {
		r.URL.Path = "/1"
	}
	// Convert page number to bignum
	count, success := new(big.Int).SetString(r.URL.Path[12:], 0)

	if !success {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Make sure page number cannot be negative or 0
	count.Abs(count)
	if count.Cmp(one) == -1 {
		count.SetInt64(1)
	}

	// Make sure we're not above page count
	if count.Cmp(total) > 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	aComputed := computeSingle(count)
	fmt.Fprintf(w,JsonKeyTemplate, aComputed.private, aComputed.number, aComputed.compressed, aComputed.uncompressed)
}

func RedirectRequest(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path[36:]

	wif, err := btcutil.DecodeWIF(key)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	page, _ := new(big.Int).DivMod(new(big.Int).SetBytes(wif.PrivKey.D.Bytes()), big.NewInt(ResultsPerPage), big.NewInt(ResultsPerPage))
	page.Add(page, one)

	fragment, _ := btcutil.NewWIF(wif.PrivKey, &chaincfg.MainNetParams, false)

	http.Redirect(w, r, "/"+page.String()+"#"+fragment.String(), http.StatusTemporaryRedirect)
}

func main() {
	http.HandleFunc("/", PageRequest)
	http.HandleFunc("/warning:understand-how-this-works!/", RedirectRequest)
	http.HandleFunc("/jsonSingle/", JsonSingleRequest)
	log.Println("Listening")
	log.Fatal(http.ListenAndServe(":8085", nil))
}
