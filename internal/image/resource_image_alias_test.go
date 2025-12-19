package image_test

import (
	"fmt"
	"regexp"
	"testing"

	petname "github.com/dustinkirkland/golang-petname"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/lxc/terraform-provider-incus/internal/acctest"
)

func TestAccImageAlias_basic(t *testing.T) {
	aliasName := petname.Generate(2, "-")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// First, create a base image to reference
				Config: testAccImageAlias_setup(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("incus_image.base", "source_image.remote", "images"),
					resource.TestCheckResourceAttr("incus_image.base", "source_image.name", "alpine/edge"),
				),
			},
			{
				// Then create an alias pointing to it
				Config: testAccImageAlias_basic(aliasName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("incus_image_alias.alias1", "name", aliasName),
					resource.TestCheckResourceAttr("incus_image_alias.alias1", "description", "Test alias"),
					resource.TestCheckResourceAttrSet("incus_image_alias.alias1", "fingerprint"),
					resource.TestCheckResourceAttrSet("incus_image_alias.alias1", "resource_id"),
				),
			},
			{
				// Verify import
				ResourceName:      "incus_image_alias.alias1",
				ImportStateId:     aliasName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccImageAlias_update(t *testing.T) {
	aliasName := petname.Generate(2, "-")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Create base images and alias
				Config: testAccImageAlias_update_initial(aliasName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("incus_image_alias.alias1", "name", aliasName),
					resource.TestCheckResourceAttr("incus_image_alias.alias1", "description", "Initial description"),
				),
			},
			{
				// Update the alias description
				Config: testAccImageAlias_update_changed(aliasName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("incus_image_alias.alias1", "name", aliasName),
					resource.TestCheckResourceAttr("incus_image_alias.alias1", "description", "Updated description"),
				),
			},
		},
	})
}

func TestAccImageAlias_move(t *testing.T) {
	aliasName := petname.Generate(2, "-")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Create two images and point alias to first
				Config: testAccImageAlias_move_image1(aliasName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("incus_image_alias.alias1", "name", aliasName),
					resource.TestCheckResourceAttrPair("incus_image_alias.alias1", "fingerprint", "incus_image.base1", "fingerprint"),
				),
			},
			{
				// Move alias to second image
				Config: testAccImageAlias_move_image2(aliasName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("incus_image_alias.alias1", "name", aliasName),
					resource.TestCheckResourceAttrPair("incus_image_alias.alias1", "fingerprint", "incus_image.base2", "fingerprint"),
				),
			},
		},
	})
}

func TestAccImageAlias_conflictDetection(t *testing.T) {
	aliasName := petname.Generate(2, "-")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Create an image with a built-in alias
				Config: testAccImageAlias_withBuiltInAlias(aliasName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("incus_image.base", "alias.0.name", aliasName),
				),
			},
			{
				// Try to create incus_image_alias with the same name - should fail
				Config:      testAccImageAlias_conflictingAlias(aliasName),
				ExpectError: regexp.MustCompile(fmt.Sprintf(`Alias %q already exists`, regexp.QuoteMeta(aliasName))),
			},
		},
	})
}

func TestAccImageAlias_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Empty alias name should fail validation
				Config:      testAccImageAlias_emptyName(),
				ExpectError: regexp.MustCompile(`string length must be at least 1`),
			},
			{
				// Empty fingerprint should fail validation
				Config:      testAccImageAlias_emptyFingerprint(),
				ExpectError: regexp.MustCompile(`string length must be at least 1`),
			},
		},
	})
}

// Test configuration functions

func testAccImageAlias_setup() string {
	return `
resource "incus_image" "base" {
  source_image = {
    remote = "images"
    name   = "alpine/edge"
  }
}
`
}

func testAccImageAlias_basic(aliasName string) string {
	return fmt.Sprintf(`
resource "incus_image" "base" {
  source_image = {
    remote = "images"
    name   = "alpine/edge"
  }
}

resource "incus_image_alias" "alias1" {
  name        = "%s"
  fingerprint = incus_image.base.fingerprint
  description = "Test alias"
}
`, aliasName)
}

func testAccImageAlias_update_initial(aliasName string) string {
	return fmt.Sprintf(`
resource "incus_image" "base" {
  source_image = {
    remote = "images"
    name   = "alpine/edge"
  }
}

resource "incus_image_alias" "alias1" {
  name        = "%s"
  fingerprint = incus_image.base.fingerprint
  description = "Initial description"
}
`, aliasName)
}

func testAccImageAlias_update_changed(aliasName string) string {
	return fmt.Sprintf(`
resource "incus_image" "base" {
  source_image = {
    remote = "images"
    name   = "alpine/edge"
  }
}

resource "incus_image_alias" "alias1" {
  name        = "%s"
  fingerprint = incus_image.base.fingerprint
  description = "Updated description"
}
`, aliasName)
}

func testAccImageAlias_move_image1(aliasName string) string {
	return fmt.Sprintf(`
resource "incus_image" "base1" {
  source_image = {
    remote = "images"
    name   = "alpine/edge"
  }
}

resource "incus_image" "base2" {
  source_image = {
    remote = "images"
    name   = "alpine/3.19"
  }
}

resource "incus_image_alias" "alias1" {
  name        = "%s"
  fingerprint = incus_image.base1.fingerprint
  description = "Points to first image"
}
`, aliasName)
}

func testAccImageAlias_move_image2(aliasName string) string {
	return fmt.Sprintf(`
resource "incus_image" "base1" {
  source_image = {
    remote = "images"
    name   = "alpine/edge"
  }
}

resource "incus_image" "base2" {
  source_image = {
    remote = "images"
    name   = "alpine/3.19"
  }
}

resource "incus_image_alias" "alias1" {
  name        = "%s"
  fingerprint = incus_image.base2.fingerprint
  description = "Points to second image"
}
`, aliasName)
}

func testAccImageAlias_withBuiltInAlias(aliasName string) string {
	return fmt.Sprintf(`
resource "incus_image" "base" {
  source_image = {
    remote = "images"
    name   = "alpine/edge"
  }

  alias {
    name        = "%s"
    description = "Built-in alias"
  }
}
`, aliasName)
}

func testAccImageAlias_conflictingAlias(aliasName string) string {
	return fmt.Sprintf(`
resource "incus_image" "base" {
  source_image = {
    remote = "images"
    name   = "alpine/edge"
  }

  alias {
    name        = "%s"
    description = "Built-in alias"
  }
}

# This should fail because the alias already exists
resource "incus_image_alias" "conflict" {
  name        = "%s"
  fingerprint = incus_image.base.fingerprint
  description = "Trying to take over existing alias"
}
`, aliasName, aliasName)
}

func testAccImageAlias_emptyName() string {
	return `
resource "incus_image" "base" {
  source_image = {
    remote = "images"
    name   = "alpine/edge"
  }
}

resource "incus_image_alias" "invalid" {
  name        = ""
  fingerprint = incus_image.base.fingerprint
  description = "Invalid - empty name"
}
`
}

func testAccImageAlias_emptyFingerprint() string {
	return `
resource "incus_image_alias" "invalid" {
  name        = "test-alias"
  fingerprint = ""
  description = "Invalid - empty fingerprint"
}
`
}
