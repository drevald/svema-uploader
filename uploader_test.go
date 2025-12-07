package main

import (
	"testing"
	"fmt"
	"os"
)


func TestOutput(t *testing.T) {
	fileDate,_ := getShotCreationDate("shot-without-exif-date.jpg")
	fileDate1,_ := getFileCreationDate("shot-without-exif-date.jpg")

	exifDate, _ := getShotCreationDate("shot-with-exif-date.jpg")
	fileLastModifiedDate, _ := getLastModifiedDate("shot-with-exif-date.jpg")
	fileCreationDate, _ := getFileCreationDate("shot-with-exif-date.jpg")
	fmt.Printf("File date is %s", fileDate)
	fmt.Printf("Exif date is %s", exifDate)
	fmt.Printf("File last modified date if exif present %s", fileLastModifiedDate)
	fmt.Printf("File created date if exif present %s", fileCreationDate)
	fmt.Printf("File created date if exif absent %s", fileDate1)

}

func TestFileDate(t *testing.T) {

	filename := "shot-without-exif-date.jpg"
	fmt.Printf("Reading %s...\n", filename)

	tm, err := getShotCreationDate(filename)
	if err != nil {
		t.Fatalf("could not extract DateTimeOriginal: %v", err)
	} 
	fmt.Printf("Date retrieved %s", tm)

	expected := "2005-06-16"
	actual := fmt.Sprintf("%d-%02d-%02d", tm.Year(), tm.Month(), tm.Day())

	if actual != expected {
		t.Errorf("DateTimeOriginal mismatch: got %s, want %s", actual, expected)
	} else {
		fmt.Println("OK")
	}
}

func TestExifDate(t *testing.T) {

	filename := "shot-with-exif-date.jpg"
	fmt.Printf("Reading %s...\n", filename)

	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("could not read file: %v", err)
	}

	tm, err := getExifDate(data)
	if err != nil {
		t.Fatalf("could not extract DateTimeOriginal: %v", err)
	}
	fmt.Printf("Date retrieved %s", tm)

	expected := "2016-09-18"
	actual := fmt.Sprintf("%d-%02d-%02d", tm.Year(), tm.Month(), tm.Day())

	if actual != expected {
		t.Errorf("DateTimeOriginal mismatch: got %s, want %s", actual, expected)
	} else {
		fmt.Println("OK")
	}

}


func TestTiff(t *testing.T) {

	filename := "shot.tif"
	fmt.Printf("Reading %s...\n", filename)

	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("could not read file: %v", err)
	}

	tm, err := getExifDate(data)
	if err != nil {
		t.Fatalf("could not extract DateTimeOriginal: %v", err)
	}
	fmt.Printf("Date retrieved %s", tm)

	expected := "2016-09-18"
	actual := fmt.Sprintf("%d-%02d-%02d", tm.Year(), tm.Month(), tm.Day())

	if actual != expected {
		t.Errorf("DateTimeOriginal mismatch: got %s, want %s", actual, expected)
	} else {
		fmt.Println("OK")
	}

}