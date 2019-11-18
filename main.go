package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/go-gl/gl/v3.2-core/gl"
	"github.com/go-gl/glfw/v3.2/glfw"
	"github.com/golang-ui/nuklear/nk"
)

const (
	pad              = 2
	maxVertexBuffer  = 512 * 1024
	maxElementBuffer = 128 * 1024
)

var (
	tree                      Folder
	cellWidth                 int32
	cellHeight                float32
	composeServerURL          []byte
	composeServerLogin        []byte
	composeServerPassword     []byte
	composeCreateFolderName   []byte
	composeCreateBookmarkName []byte
	composeCreateBookmarkURL  []byte
	errorMsg                  string

	serverURL string

	// parentId of the folder to be created
	folderCreated int
	// parentId of the bookmark to be created
	bookmarkCreated int
)

// Tag
type Tag struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

// Folder containing the bookmarks
type Folder struct {
	Id                int         `json:"id"`
	Title             string      `json:"title"`
	Parent            *Folder     `json:"parent"`
	Folders           []*Folder   `json:"folders"`
	Bookmarks         []*Bookmark `json:"bookmarks"`
	NbChildrenFolders int         `json:"nbchildrenfolders"`
}

// Bookmark
type Bookmark struct {
	Id      int     `json:"id"`
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Favicon string  `json:"favicon"` // base64 encoded image
	Starred bool    `json:"starred"`
	Folder  *Folder `json:"folder"` // reference to the folder to help
	Tags    []*Tag  `json:"tags"`
}

func init() {
	composeServerURL = make([]byte, 512)
	composeServerLogin = make([]byte, 512)
	composeServerPassword = make([]byte, 512)
	composeCreateFolderName = make([]byte, 512)
	composeCreateBookmarkName = make([]byte, 512)
	composeCreateBookmarkURL = make([]byte, 512)
	copy(composeServerURL[:], "server URL")
	copy(composeServerLogin[:], "login")
	copy(composeServerPassword[:], "password")
	copy(composeCreateBookmarkName[:], "name")
	copy(composeCreateBookmarkURL[:], "URL")

	folderCreated = -1
	bookmarkCreated = -1

	getRemoteBookmarks("http://localhost:8081")
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func getRemoteBookmarks(url string) {
	serverURL = strings.TrimSpace(url)

	var client http.Client
	req, err := http.NewRequest("GET", serverURL+"/getTree/", nil)
	if err != nil {
		errorMsg = err.Error()
		return
	}

	req.Header.Add("Authorization", "Basic "+basicAuth(string(composeServerLogin), string(composeServerPassword)))
	resp, err := client.Do(req)
	if err != nil {
		errorMsg = err.Error()
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errorMsg = err.Error()
		return
	}

	err = json.Unmarshal(body, &tree)
	if err != nil {
		errorMsg = err.Error()
	}
}

func createFolder(folderName string, parentId int) {
	var client http.Client
	url := fmt.Sprintf("%s/addFolder/?folderName=%s&parentId=%d", serverURL, folderName, parentId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		errorMsg = err.Error()
		return
	}

	req.Header.Add("Authorization", "Basic "+basicAuth(string(composeServerLogin), string(composeServerPassword)))
	_, err = client.Do(req)
	if err != nil {
		errorMsg = err.Error()
		return
	}
}

// https://gist.github.com/hyg/9c4afcd91fe24316cbf0
func openbrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}
}

func buildTree(f *Folder, ctx *nk.Context) {

	if nk.NkTreePushHashed(ctx, nk.TreeTab, f.Title, nk.Minimized, f.Title, int32(len(f.Title)), 0) > 0 {
		nk.NkLayoutRowTemplateBegin(ctx, 30)
		nk.NkLayoutRowTemplatePushStatic(ctx, 125)
		nk.NkLayoutRowTemplatePushStatic(ctx, 125)
		nk.NkLayoutRowTemplateEnd(ctx)
		// add bookmark button
		if nk.NkButtonLabel(ctx, "new bookmark") > 0 {
			bookmarkCreated = f.Id
			folderCreated = -1
		}
		// add folder button
		if nk.NkButtonLabel(ctx, "new folder") > 0 {
			folderCreated = f.Id
			bookmarkCreated = -1
		}
		if folderCreated == f.Id {
			// folder creation form
			nk.NkLayoutRowTemplateBegin(ctx, 30)
			nk.NkLayoutRowTemplatePushVariable(ctx, 200)
			nk.NkLayoutRowTemplatePushStatic(ctx, 125)
			nk.NkLayoutRowTemplateEnd(ctx)
			nk.NkEditStringZeroTerminated(
				ctx,
				nk.EditField|nk.EditSelectable|nk.EditClipboard|nk.EditNoHorizontalScroll|nk.TextEditSingleLine,
				composeCreateFolderName,
				256,
				nk.NkFilterAscii)
			if nk.NkButtonLabel(ctx, "ok") > 0 {
				composeCreateFolderName = bytes.Trim(composeCreateFolderName, "\x00")
				createFolder(string(composeCreateFolderName), f.Id)
				folderCreated = -1

				//refreshing tree
				getRemoteBookmarks(serverURL)
			}
		}
		if bookmarkCreated == f.Id {
			// bookmark creation form
			nk.NkLayoutRowTemplateBegin(ctx, 30)
			nk.NkLayoutRowTemplatePushVariable(ctx, 200)
			nk.NkLayoutRowTemplatePushVariable(ctx, 200)
			nk.NkLayoutRowTemplatePushStatic(ctx, 125)
			nk.NkLayoutRowTemplateEnd(ctx)
			nk.NkEditStringZeroTerminated(
				ctx,
				nk.EditField|nk.EditSelectable|nk.EditClipboard|nk.EditNoHorizontalScroll|nk.TextEditSingleLine,
				composeCreateBookmarkName,
				256,
				nk.NkFilterAscii)
			nk.NkEditStringZeroTerminated(
				ctx,
				nk.EditField|nk.EditSelectable|nk.EditClipboard|nk.EditNoHorizontalScroll|nk.TextEditSingleLine,
				composeCreateBookmarkURL,
				256,
				nk.NkFilterAscii)
			if nk.NkButtonLabel(ctx, "ok") > 0 {
				fmt.Println("bookmark created")
				bookmarkCreated = -1
			}
		}
		for _, childb := range f.Bookmarks {
			nk.NkLayoutRowTemplateBegin(ctx, 25)
			nk.NkLayoutRowTemplatePushDynamic(ctx)
			nk.NkLayoutRowTemplatePushStatic(ctx, 30)
			nk.NkLayoutRowTemplateEnd(ctx)
			nk.NkLabel(ctx, childb.Title, nk.TextAlignRight)
			if nk.NkButtonSymbol(ctx, nk.SymbolCircleSolid) > 0 {
				openbrowser(childb.URL)
			}
		}
		for _, childf := range f.Folders {
			buildTree(childf, ctx)
		}

		nk.NkTreePop(ctx)
	}
}

func draw(win *glfw.Window, ctx *nk.Context) {
	//
	// define GUI
	//
	// create frame = single snapshot of the GUI
	nk.NkPlatformNewFrame()
	width, height := win.GetSize()
	bounds := nk.NkRect(0, 0, float32(width), float32(height))

	// any code between nk.NkBegin() and nk.NkEnd() will be part of our UI update for the frame we just started
	update := nk.NkBegin(ctx, "", bounds, nk.WindowScrollAutoHide)

	if update > 0 {
		// an event occured
		cellWidth = int32(width - pad*2)
		cellHeight = float32(height)

		nk.NkLayoutRowStatic(ctx, 0, cellWidth, 1)

		// building the log label
		if errorMsg != "" {
			nk.NkLabel(ctx, errorMsg, nk.TextAlignLeft|nk.TextAlignMiddle)
		} else {
			nk.NkLabel(ctx, "", nk.TextAlignLeft|nk.TextAlignMiddle)
		}

		// building GoBKM URL input field
		nk.NkLayoutRowTemplateBegin(ctx, 35)
		nk.NkLayoutRowTemplatePushStatic(ctx, 100)
		nk.NkLayoutRowTemplatePushVariable(ctx, 200)
		nk.NkLayoutRowTemplatePushStatic(ctx, 80)
		nk.NkLayoutRowTemplateEnd(ctx)
		nk.NkLabel(ctx, "server", nk.TextAlignLeft|nk.TextAlignMiddle)
		nk.NkEditStringZeroTerminated(
			ctx,
			nk.EditField|nk.EditSelectable|nk.EditClipboard|nk.EditNoHorizontalScroll|nk.TextEditSingleLine,
			composeServerURL,
			256,
			nk.NkFilterAscii)
		if nk.NkButtonLabel(ctx, "connect") > 0 {
			errorMsg = ""
			composeServerURL = bytes.Trim(composeServerURL, "\x00")
			composeServerLogin = bytes.Trim(composeServerLogin, "\x00")
			composeServerPassword = bytes.Trim(composeServerPassword, "\x00")
			getRemoteBookmarks(string(composeServerURL))
		}

		// building GoBKM username input field
		nk.NkLayoutRowTemplateBegin(ctx, 35)
		nk.NkLayoutRowTemplatePushStatic(ctx, 100)
		nk.NkLayoutRowTemplatePushVariable(ctx, 200)
		nk.NkLayoutRowTemplateEnd(ctx)
		nk.NkLabel(ctx, "login", nk.TextAlignLeft|nk.TextAlignMiddle)
		nk.NkEditStringZeroTerminated(
			ctx,
			nk.EditField|nk.EditSelectable|nk.EditClipboard|nk.EditNoHorizontalScroll|nk.TextEditSingleLine,
			composeServerLogin,
			256,
			nk.NkFilterDefault)

		// building GoBKM password input field
		nk.NkLayoutRowTemplateBegin(ctx, 35)
		nk.NkLayoutRowTemplatePushStatic(ctx, 100)
		nk.NkLayoutRowTemplatePushVariable(ctx, 200)
		nk.NkLayoutRowTemplateEnd(ctx)
		nk.NkLabel(ctx, "password", nk.TextAlignLeft|nk.TextAlignMiddle)
		nk.NkEditStringZeroTerminated(
			ctx,
			nk.EditField|nk.EditSelectable|nk.EditClipboard|nk.EditNoHorizontalScroll|nk.TextEditSingleLine,
			composeServerPassword,
			256,
			nk.NkFilterDefault)

		// building the tree
		buildTree(&tree, ctx)

	}
	nk.NkEnd(ctx)

	//
	// draw GUI to viewport
	//
	// ask OpenGL to render the GUI
	gl.Viewport(0, 0, int32(width), int32(height))
	gl.Clear(gl.COLOR_BUFFER_BIT)
	gl.ClearColor(0x10, 0x10, 0x10, 0xff)
	nk.NkPlatformRender(nk.AntiAliasingOn, maxVertexBuffer, maxElementBuffer)
	win.SwapBuffers()
}

func init() {
	runtime.LockOSThread()
}

func main() {
	// initialize glfw
	glfw.Init()
	// create main windows
	win, _ := glfw.CreateWindow(120, 200, "GoBkm", nil, nil)
	// make the new window current so that its context is used in the following code
	win.MakeContextCurrent()
	// initialize opengl context
	gl.Init()
	// create context
	ctx := nk.NkPlatformInit(win, nk.PlatformInstallCallbacks)
	// set default font
	atlas := nk.NewFontAtlas()
	nk.NkFontStashBegin(&atlas)
	font := nk.NkFontAtlasAddDefault(atlas, 18, nil)
	nk.NkFontStashEnd()
	nk.NkStyleSetFont(ctx, font.Handle())

	quit := make(chan struct{}, 1)
	ticker := time.NewTicker(time.Second / 30)
	// loop that handles GUI refreshing and event management
	for {
		select {
		case <-quit:
			nk.NkPlatformShutdown()
			glfw.Terminate()
			ticker.Stop()
			return
		case <-ticker.C:
			if win.ShouldClose() {
				close(quit)
				continue
			}
			glfw.PollEvents()
			// GUI definition
			draw(win, ctx)
		}
	}
}
