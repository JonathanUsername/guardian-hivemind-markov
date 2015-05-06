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


This file was slightly changed by me, Jon King

*/
package main

import (
    "bytes"
    "sync"
    "os/exec"
    "bufio"
    "fmt"
    "io"
    "math/rand"
    "strings"
    "time"
    "net/http"
    "strconv"
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


func Scrape(query string, wg *sync.WaitGroup, c *Chain) {
    // Run command, then plug the command's Stdout into the expectant maw of Build's io.Reader
    fmt.Println(query)
    cmd := exec.Command("./scrape", "-q", query)
    var out bytes.Buffer
    cmd.Stdout = &out
    cmd.Run()
    c.Build(&out)
    wg.Done()
}

func handler(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query().Get("q")
    wordL, err := strconv.Atoi(r.URL.Query().Get("wl"))
    check(err)
    prefixL, err := strconv.Atoi(r.URL.Query().Get("pl"))
    check(err)
    this := CreateArticle(q, wordL, prefixL)
    fmt.Fprintf(w, "%s", this)
}

func CreateArticle(query string, wordL int, prefixL int) string {
    rand.Seed(time.Now().UnixNano()) // Seed the random number generator.
    c := NewChain(prefixL)     // Initialize a new Chain.

    fmt.Println(query, wordL, prefixL)

    var wg sync.WaitGroup         // Create waitgroup for async node process
    wg.Add(1)
    Scrape(query, &wg, c)        // Then run node process
    wg.Wait()

    text := c.Generate(wordL) // Generate text.
    return text
}

func main() {
    listen()
}

func listen() {
    http.HandleFunc("/", handler)
    http.ListenAndServe(":8080", nil)
}

func check(err error) {
    if err != nil {
        panic(err)
    }
}