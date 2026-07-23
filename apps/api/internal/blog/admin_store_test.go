package blog

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestForeignKeyInUse(t *testing.T) {
	if !isForeignKeyInUse(fmt.Errorf("delete taxonomy: %w", &mysql.MySQLError{Number: 1451})) {
		t.Fatal("wrapped MySQL foreign-key error was not recognized")
	}
	if isForeignKeyInUse(&mysql.MySQLError{Number: 1062}) {
		t.Fatal("unrelated MySQL error was recognized as an in-use taxonomy")
	}
}

func TestMarkdownImageURLs(t *testing.T) {
	markdown := `
![simple](/uploads/simple.png)
![title](/uploads/titled.webp "cover title")
![angle](</uploads/path with spaces.png>)
![parentheses](/uploads/image\(edited\).png)
![reference][hero]
![collapsed][]
![shortcut]
![duplicate](/uploads/simple.png)
![external](https://cdn.example.com/external.png)

[hero]: /uploads/reference.png "reference title"
[collapsed]: /uploads/collapsed.png
[shortcut]: /uploads/shortcut.png
`
	want := []string{
		"/uploads/simple.png",
		"/uploads/titled.webp",
		"/uploads/path with spaces.png",
		"/uploads/image(edited).png",
		"/uploads/reference.png",
		"/uploads/collapsed.png",
		"/uploads/shortcut.png",
		"https://cdn.example.com/external.png",
	}
	if got := markdownImageURLs(markdown); !reflect.DeepEqual(got, want) {
		t.Fatalf("markdownImageURLs() = %#v, want %#v", got, want)
	}
}

func TestMarkdownImageURLsIgnoresLinksAndCode(t *testing.T) {
	markdown := "[link](/uploads/not-an-image.png)\n\n`![inline](/uploads/code.png)`\n\n```md\n![block](/uploads/code-block.png)\n```"
	if got := markdownImageURLs(markdown); len(got) != 0 {
		t.Fatalf("markdownImageURLs() = %#v, want no images", got)
	}
}
