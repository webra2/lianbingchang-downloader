// Copyright 2021 Antonio Sanchez (asanchez.dev). All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/antsanchez/go-download-web/dp"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/antsanchez/go-download-web/scraper"
	"github.com/antsanchez/go-download-web/sitemap"
)

const CDUrl = "https://game.chronodivide.com/"

var CDResUrl = "https://gameres.chronodivide.com/"

type Flags struct {
	Domain *string

	// New Domain to be set
	NewDomain *string

	// Number of concurrent queries
	Simultaneus *int

	// Use query parameters on URLs
	UseQueries *bool

	// Path where to download the files to
	Path *string
}

func parseFlags() (flags Flags, err error) {
	flags.Domain = flag.String("u", "", "chronodivide source site URL")
	flags.NewDomain = flag.String("new", "", "New URL")
	flags.Simultaneus = flag.Int("s", 3, "Number of concurrent connections")
	flags.UseQueries = flag.Bool("q", false, "Ignore queries on URLs")
	flags.Path = flag.String("path", "./website", "Local path for downloaded files")
	flag.Parse()

	if *flags.Domain == "" {
		*flags.Domain = CDUrl
	}

	if *flags.Simultaneus <= 0 {
		err = errors.New("the number of concurrent connections be at least 1'")
		return
	}

	log.Println("Domain:", *flags.Domain)
	if *flags.NewDomain != "" {
		log.Println("New Domain: ", *flags.NewDomain)
	}
	log.Println("Simultaneus:", *flags.Simultaneus)
	log.Println("Use Queries:", *flags.UseQueries)

	return
}

func main() {

	flags, err := parseFlags()
	if err != nil {
		log.Fatal(err)
	}

	err = updateServiceINI(*flags.Domain, *flags.Path)
	if err != nil {
		log.Println(err)
	}
	err = checkVersion(*flags.Domain, *flags.Path)
	if err != nil {
		log.Println(err)
	}
	scanUrls := dp.RecordNetwork(*flags.Domain)

	// Create directory for downloaded website
	err = os.MkdirAll(*flags.Path, 0755)
	if err != nil {
		log.Println(*flags.Path)
		log.Fatal(err)
	}

	scanning := make(chan int, *flags.Simultaneus) // Semaphore
	newLinks := make(chan []scraper.Links, 100000) // New links to scan
	pages := make(chan scraper.Page, 100000)       // Pages scanned
	attachments := make(chan []string, 100000)     // Attachments
	started := make(chan int, 100000)              // Crawls started
	finished := make(chan int, 100000)             // Crawls finished
	var version string

	var indexed, forSitemap, files []string

	seen := make(map[string]bool)

	start := time.Now()

	defer func() {
		close(newLinks)
		close(pages)
		close(started)
		close(finished)
		close(scanning)

		log.Printf("\nDuration: %s\n", time.Since(start))
		log.Printf("Number of pages: %6d\n", len(indexed))
	}()

	// Do First call to domain
	resp, err := http.Get(*flags.Domain)
	if err != nil {
		log.Println("Domain could not be reached!")
		return
	}
	defer resp.Body.Close()

	s := scraper.Scraper{
		OldDomain:  *flags.Domain,
		NewDomain:  *flags.NewDomain,
		Root:       resp.Request.URL.String(),
		Path:       *flags.Path,
		UseQueries: *flags.UseQueries,
	}

	log.Println("\n Start download site...")

	// Take the links from the startsite
	s.TakeLinks(*flags.Domain, started, finished, scanning, newLinks, pages, attachments, &version)
	seen[*flags.Domain] = true

	founded := false
	for {
		select {
		case links := <-newLinks:
			for _, link := range links {
				if !seen[link.Href] {
					seen[link.Href] = true
					go s.TakeLinks(link.Href, started, finished, scanning, newLinks, pages, attachments, &version)
				}
			}
			founded = true
		case page := <-pages:
			founded = true
			if !s.IsURLInSlice(page.URL, indexed) {
				indexed = append(indexed, page.URL)
				go func() {
					err := s.SaveHTML(page.URL, page.HTML)
					if err != nil {
						log.Println(err)
					}
				}()
			}

			if !page.NoIndex {
				if !s.IsURLInSlice(page.URL, forSitemap) {
					forSitemap = append(forSitemap, page.URL)
				}
			}
		case attachment := <-attachments:
			founded = true
			for _, link := range attachment {
				if !s.IsURLInSlice(link, files) {
					files = append(files, link)
				}
			}
		}

		// Break the for loop once all scans are finished
		if founded && len(started) > 0 && len(scanning) == 0 && len(started) == len(finished) && len(attachments) == 0 && len(pages) == 0 && len(newLinks) == 0 {
			break
		}
	}

	log.Println("\nFinished scraping the site, version :", version)

	for _, scanUrl := range scanUrls {
		files = append(files, s.RemoveQuery(scanUrl))
	}
	// other files
	otherFiles := []string{
		"/lib/ffmpeg-core.js","/lib/ffmpeg-core.wasm","/lib/ffmpeg-core.worker.js",
		"/dist/ffmpeg.min.js","/dist/ffmped-core.js","/dist/ffmpeg-core.wasm","/servers.ini","/res/locale/zh-CN.json"}
	for _,ofile := range otherFiles {
		files = append(files, fmt.Sprintf("%s/%s", *flags.Domain,ofile))
	}
	

	log.Println("\nDownloading attachments...", len(files))
	for _, attachedFile := range files {
		if !strings.Contains(attachedFile, *flags.Domain) && !strings.Contains(attachedFile, CDResUrl) {
			continue
		}
		// skip mix files
		if strings.Contains(attachedFile,".mix") {
			continue
		}
		s.SaveAttachment(attachedFile)
		if strings.Contains(attachedFile, "manifest.json") {
			moreAttachments := s.GetManifest(attachedFile)
			for _, link := range moreAttachments {
				if !s.IsURLInSlice(link, files) {
					log.Println("Appended Manifest: ", link)
					files = append(files, link)
					go func() {
						err := s.SaveAttachment(link)
						if err != nil {
							log.Println(err)
						}
					}()
				}
			}
		}
		if strings.Contains(attachedFile, ".css") {
			moreAttachments := s.GetInsideAttachments(attachedFile)
			for _, link := range moreAttachments {
				if !s.IsURLInSlice(link, files) {
					log.Println("Appended: ", link)
					files = append(files, link)
					go func() {
						err := s.SaveAttachment(link)
						if err != nil {
							log.Println(err)
						}
					}()
				}
			}
		}
	}

	log.Println("Creating Sitemap...")
	err = sitemap.CreateSitemap(forSitemap, *flags.Path)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Finished.")
}
