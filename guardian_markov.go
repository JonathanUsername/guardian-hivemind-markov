// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Generating random text: a Markov chain algorithm

Based on the program presented in the "Design and Implementation" chapter
of The Practice of Programming (Kernighan and Pike, Addison-Wesley 1999).
See also Computer Recreations, Scientific American 260, 122 - 125 (1989).

A Markov chain algorithm generates text by creating a statistical model of
potential textual suffixes for a given prefix. Consider this text:

    I am not a number! I am a free man!

Our Markov chain algorithm would arrange this text into this set of prefixes
and suffixes, or "chain": (This table assumes a prefix length of two words.)

    Prefix       Suffix

    "" ""        I
    "" I         am
    I am         a
    I am         not
    a free       man!
    am a         free
    am not       a
    a number!    I
    number! I    am
    not a        number!

To generate text using this table we select an initial prefix ("I am", for
example), choose one of the suffixes associated with that prefix at random
with probability determined by the input statistics ("a"),
and then create a new prefix by removing the first word from the prefix
and appending the suffix (making the new prefix is "am a"). Repeat this process
until we can't find any suffixes for the current prefix or we exceed the word
limit. (The word limit is necessary as the chain table may contain cycles.)

Our version of this program reads text from standard input, parsing it into a
Markov chain, and writes generated text to standard output.
The prefix and output lengths can be specified using the -prefix and -words
flags on the command-line.


This file was altered by me, Jon King, jonathan.j.king@gmail.com

*/
package main

import (
    "sync"
    "bufio"
    "fmt"
    "io"
    "math/rand"
    "strings"
    "time"
    "net/http"
    "strconv"
    "github.com/jmoiron/jsonq"
    "encoding/json"
    "io/ioutil"
    "os"
    "regexp"
)

// Prefix is a Markov chain prefix of one or more words.
type Prefix []string

// String returns the Prefix as a string (for use as a map key).
func (p Prefix) String() string {
    return strings.Join(p, " ")
}

// Shift removes the first word from the Prefix and appends the given word.
func (p Prefix) Shift(word string) {
    copy(p, p[1:])
    p[len(p)-1] = word
}

// Chain contains a map ("chain") of prefixes to a list of suffixes.
// A prefix is a string of prefixLen words joined with spaces.
// A suffix is a single word. A prefix can have multiple suffixes.
type Chain struct {
    chain     map[string][]string
    prefixLen int
}

// NewChain returns a new Chain with prefixes of prefixLen words.
func NewChain(prefixLen int) *Chain {
    return &Chain{make(map[string][]string), prefixLen}
}

// Build reads text from the provided Reader and
// parses it into prefixes and suffixes that are stored in Chain.
func (c *Chain) Build(r io.Reader) {
    br := bufio.NewReader(r)
    p := make(Prefix, c.prefixLen)
    for {
        var s string
        if _, err := fmt.Fscan(br, &s); err != nil {
            break
        }
        key := p.String()
        c.chain[key] = append(c.chain[key], s)
        p.Shift(s)
    }
}

// Generate returns a string of at most n words generated from Chain.
func (c *Chain) Generate(n int) string {
    p := make(Prefix, c.prefixLen)
    var words []string
    for i := 0; i < n; i++ {
        choices := c.chain[p.String()]
        if len(choices) == 0 {
            break
        }
        next := choices[rand.Intn(len(choices))]
        words = append(words, next)
        p.Shift(next)
    }
    return strings.Join(words, " ")
}

func getFields(stringbody string, field string) string {
    results, err := fromJson(stringbody).Array("response", "results")
    check(err)
    var wholebody string
    regex, _ := regexp.Compile("<[^>]+>")
    for _, each := range results {
        body := each.(map[string]interface{})["fields"].(map[string]interface{})[field]
        cleaned := regex.ReplaceAllString(body.(string), " ")
        wholebody += cleaned
    }
    return wholebody
}

func Scrape(query string, wg *sync.WaitGroup) string {
    url := fmt.Sprintf("http://content.guardianapis.com/search?show-fields=body,headline&page-size=200&api-key=%s&q=%s", getKey(), query)
    resp, err := http.Get(url)
    check(err)
    defer resp.Body.Close()

    // why can't I find a go-based json parser that works with io.Reader rather than strings? For purity's sake I'm doing this rather than piping to jq, but I think this is a bottleneck...
    body, err := ioutil.ReadAll(resp.Body)

    wg.Done()
    return string(body)
}

type article_response struct {
    Headline    string
    Body        string
}

func CreateArticle(query string, wordL int, prefixL int) article_response {
    rand.Seed(time.Now().UnixNano()) // Seed the random number generator.

    fmt.Println(query, wordL, prefixL)

    var wg sync.WaitGroup         // Create waitgroup for scrape process
    wg.Add(1)
    scrape := Scrape(query, &wg)
    wg.Wait()

    // text = buildPart("headline", scrape, 2, 20)   // this turned out to be a bit too random

    // build headline
    arr, _ := fromJson(scrape).Array("response", "results")
    random_index := rand.Intn(len(arr))
    headline, _ := fromJson(scrape).String("response", "results", strconv.Itoa(random_index), "fields", "headline")

    // build body
    body := buildPart("body", scrape, prefixL, wordL)

    artr := article_response{Headline: headline, Body: body}

    return artr
}

func buildPart(part string, scrape string, prefixL int, wordL int) string {
    c := NewChain(prefixL)
    cleanbody := getFields(scrape, part)
    b := io.Reader(strings.NewReader(cleanbody))
    c.Build(b)
    text := c.Generate(wordL)
    return text
}

func main() {
    listen()
}

func handler(w http.ResponseWriter, r *http.Request) {
    fmt.Println("requestURI:  " + string(r.RequestURI))
    q := r.URL.Query().Get("q")
    wordL, err := strconv.Atoi(r.URL.Query().Get("wl"))
    if err != nil {
        wordL = 100
    }
    prefixL, err := strconv.Atoi(r.URL.Query().Get("pl"))
    if err != nil {
        prefixL = 2
    }
    article := CreateArticle(q, wordL, prefixL)
    jsonresp, err := json.Marshal(article)
    check(err)
    fmt.Println("returning!")
    fmt.Fprintf(w, "%s", jsonresp)
}

func listen() {
    fmt.Println("listening!")
    http.HandleFunc("/write", handler)
    http.ListenAndServe(":8080", nil)
}

func getKey() string {
    file, e := ioutil.ReadFile("../keys.private.json")
    if e != nil {
        fmt.Printf("File error: %v\n", e)
        os.Exit(1)
    }
    apikey, err := fromJson(string(file)).String("key")
    check(err)
    return apikey
}

func fromJson(js string) *jsonq.JsonQuery {
    // usage: var, err := fromJson(json).String("value", "nestedvalue", "somearray, "0")
    data := map[string]interface{}{}
    dec := json.NewDecoder(strings.NewReader(js))
    dec.Decode(&data)
    jq := jsonq.NewQuery(data)
    return jq
}

func check(err error) {
    if err != nil {
        panic(err)
    }
}