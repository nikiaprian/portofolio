package imagecaption



import (

	"strings"

	"testing"



	"github.com/nikiaprian/tirtayasa/internal/entity"

)



func TestBuildHeroCaptionInnerHTML(t *testing.T) {

	img := "https://cdn.example/x.jpg"

	src := "https://news.example.com/article"

	post := entity.Post{

		Title:             "Judul Test",

		Image:             &img,

		ImageOrigin:       entity.ImageOriginWebSearch,

		ImageOriginDetail: entity.WebImageProviderPexels,

	}

	got := BuildHeroCaptionInnerHTML(true, entity.ImageCaptionModeDefault, "", post, "Judul Test")

	if !strings.Contains(got, "Foto: Judul Test.") {

		t.Fatalf("web_search caption: %q", got)

	}

	if !strings.Contains(got, "Pexels") {

		t.Fatalf("expected Pexels credit: %q", got)

	}



	post.ImageOrigin = entity.ImageOriginAI

	got = BuildHeroCaptionInnerHTML(true, entity.ImageCaptionModeDefault, "", post, "Judul Test")

	if got != "Foto: Ilustrasi Judul Test." {

		t.Fatalf("ai caption: %q", got)

	}



	post.ImageOrigin = entity.ImageOriginScrape

	post.ImageOriginDetail = ""

	post.Source = &src

	got = BuildHeroCaptionInnerHTML(true, entity.ImageCaptionModeDefault, "", post, "Judul Test")

	if got != "Foto: Judul Test." {

		t.Fatalf("scrape caption: %q", got)

	}



	if BuildHeroCaptionInnerHTML(false, entity.ImageCaptionModeDefault, "", post, "") != "" {

		t.Fatal("disabled should be empty")

	}

	gotManual := BuildHeroCaptionInnerHTML(true, entity.ImageCaptionModeDefault, "", entity.Post{ImageOrigin: entity.ImageOriginManual, Image: &img}, "Judul Manual")

	if gotManual != "Foto: Ilustrasi Judul Manual." {

		t.Fatalf("manual upload caption: %q", gotManual)

	}



	legacy := entity.Post{Title: "Judul Lama", Image: &img}

	got = BuildHeroCaptionInnerHTML(true, entity.ImageCaptionModeDefault, "", legacy, "Judul Lama")

	if got != "Foto: Ilustrasi Judul Lama." {

		t.Fatalf("legacy empty origin: %q", got)

	}



	post.ImageOrigin = entity.ImageOriginWebSearch
	post.ImageOriginDetail = entity.WebImageProviderPexels
	customTpl := "Foto: {{title}} dari {{provider}}{{credit}}"

	got = BuildHeroCaptionInnerHTML(true, entity.ImageCaptionModeCustom, customTpl, post, "Judul Test")

	if !strings.Contains(got, "Judul Test") || !strings.Contains(got, "Pexels") {

		t.Fatalf("custom caption: %q", got)

	}

}


