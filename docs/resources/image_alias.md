# incus_image_alias

Manages an Incus image alias.

Unlike the `alias` block in `incus_image`, this resource manages aliases independently from image creation.

## Example Usage

```hcl
resource "incus_image" "alpine" {
  source_image = {
    remote = "images"
    name   = "alpine/edge"
  }
}

resource "incus_image_alias" "alpine_prod" {
  name        = "alpine/prod"
  fingerprint = incus_image.alpine.fingerprint
  description = "Production Alpine image"
}
```

## Multiple aliases for one image

```hcl
resource "incus_image_alias" "service_prod" {
  name        = "myservice/prod"
  fingerprint = "sha256:abc123..."
  description = "Production version of myservice"
}

resource "incus_image_alias" "service_latest" {
  name        = "myservice/latest"
  fingerprint = "sha256:abc123..."
  description = "Latest version of myservice"
}
```

## Moving an alias between images

```hcl
resource "incus_image" "old_image" {
  source_image = {
    remote = "images"
    name   = "ubuntu/22.04"
  }
}

resource "incus_image" "new_image" {
  source_image = {
    remote = "images"
    name   = "ubuntu/24.04"
  }
}

resource "incus_image_alias" "ubuntu_stable" {
  name        = "ubuntu/stable"
  fingerprint = incus_image.new_image.fingerprint
  description = "Stable Ubuntu release"
}
```

## Argument Reference

* `name` - **Required** - Name of the image alias. Changing this forces a new resource to be created.

* `fingerprint` - **Required** - Fingerprint of the image this alias should point to. The image must already exist on the server.

* `description` - *Optional* - Description of the image alias.

* `project` - *Optional* - Name of the project where the image alias will be created. Changing this forces a new resource to be created.

* `remote` - *Optional* - The remote in which the resource will be created. If not provided, the provider's default remote will be used. Changing this forces a new resource to be created.

## Attribute Reference

The following attributes are exported:

* `resource_id` - Unique identifier for this resource in the format `remote/project/alias_name`.

## Import

Image aliases can be imported using the alias name (provider defaults for remote/project will be applied):

```shell
terraform import incus_image_alias.example alias-name
```

Or with explicit remote and/or project:

```shell
terraform import incus_image_alias.example remote:project/alias-name
terraform import incus_image_alias.example remote/alias-name
terraform import incus_image_alias.example project/alias-name
```

`remote:` is the disambiguator: without the colon, the segment before `/` is treated as the remote and the project stays at its default. Use `remote:project/alias` when you need to set both, especially if the project name could be mistaken for a remote name.

## Notes

### Conflict Detection

If an alias already exists when creating the resource, Terraform will **always return an error**, regardless of which fingerprint it points to. This ensures explicit control over which aliases Terraform manages.

To manage an existing alias with Terraform, you must explicitly import it:

```shell
terraform import incus_image_alias.example alias-name
```

This prevents accidentally taking over aliases that may be managed elsewhere.

**Note:** Deleting this resource removes the alias only. The image itself is not deleted.
