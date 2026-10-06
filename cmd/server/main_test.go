package main

import "testing"

func TestProtectedResourceMetadataLocations(t *testing.T) {
	metadataURL, paths, err := protectedResourceMetadataLocations("https://api.yar.example/mcp")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := metadataURL, "https://api.yar.example/.well-known/oauth-protected-resource/mcp"; got != want {
		t.Fatalf("metadataURL = %q, want %q", got, want)
	}
	if len(paths) != 2 || paths[0] != "/.well-known/oauth-protected-resource/mcp" || paths[1] != "/.well-known/oauth-protected-resource" {
		t.Fatalf("paths = %#v", paths)
	}
}

func TestProtectedResourceMetadataLocationsRejectsInsecureRemoteHTTP(t *testing.T) {
	if _, _, err := protectedResourceMetadataLocations("http://api.yar.example/mcp"); err == nil {
		t.Fatal("expected insecure remote resource URL to be rejected")
	}
}

func TestProtectedResourceMetadataLocationsAllowsLoopbackHTTP(t *testing.T) {
	if _, _, err := protectedResourceMetadataLocations("http://127.0.0.1:8080/mcp"); err != nil {
		t.Fatalf("loopback resource rejected: %v", err)
	}
}

func TestParseScopes(t *testing.T) {
	got := parseScopes("yar:mcp, yar:team:run  yar:profile:read")
	if len(got) != 3 {
		t.Fatalf("scopes = %#v", got)
	}
}
