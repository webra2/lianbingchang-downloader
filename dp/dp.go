package dp

import (
	"context"
	"fmt"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"log"
	"os"
	"time"
)

func RecordNetwork(cdUrl string) []string {
	dir := os.TempDir()
	urls := make([]string, 0)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoDefaultBrowserCheck,
		//chromedp.Flag("headless", false),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("window-size", "800,600"),
		chromedp.UserDataDir(dir),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	// also set up a custom logger
	taskCtx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancel()

	// create a timeout
	taskCtx, cancel = context.WithTimeout(taskCtx, 1000*time.Second)
	defer cancel()

	// ensure that the browser process is started
	if err := chromedp.Run(taskCtx); err != nil {
		panic(err)
	}

	// listen network event
	listenForNetworkEvent(taskCtx, &urls)

	err := chromedp.Run(taskCtx,
		network.Enable(),
		chromedp.Navigate(cdUrl),
		chromedp.WaitVisible(`.message-box-content`, chromedp.ByQuery),
		chromedp.Sleep(time.Second),
		chromedp.Evaluate("document.querySelector('.close-button').click()", nil),
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("close menu")
			return nil
		}),
		chromedp.Sleep(time.Second),
		chromedp.WaitVisible(`.dialog-button`, chromedp.ByQuery),
		chromedp.Evaluate("document.querySelectorAll('.dialog-button')[1].click()", nil),
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("close dialog")
			return nil
		}),
		chromedp.Sleep(time.Second),
		chromedp.WaitVisible(".menu-button", chromedp.ByQuery),
		chromedp.QueryAfter(".menu-button", func(ctx context.Context, execCtx runtime.ExecutionContextID, nodes ...*cdp.Node) error {
			if len(nodes) < 3 {
				return fmt.Errorf("selector %q did not return any nodes", ".menu button")
			}
			return chromedp.MouseClickNode(nodes[2]).Do(ctx)
		}, chromedp.ByQueryAll),
		chromedp.Sleep(time.Second),
		chromedp.WaitVisible(".player-slot", chromedp.ByQuery),
		chromedp.Sleep(time.Second),
		chromedp.Click(".menu-button", chromedp.ByQuery),
		chromedp.Sleep(time.Second*20),
	)
	if err != nil {
		log.Println("dp error ", err)
	}
	return urls
}

// 监听
func listenForNetworkEvent(ctx context.Context, urls *[]string) {
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *network.EventRequestWillBeSent:
			*urls = append(*urls, ev.Request.URL)
		}
	})
}
