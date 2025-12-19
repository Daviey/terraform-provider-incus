package image

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lxc/incus/v6/shared/api"

	"github.com/lxc/terraform-provider-incus/internal/common"
	"github.com/lxc/terraform-provider-incus/internal/errors"
	provider_config "github.com/lxc/terraform-provider-incus/internal/provider-config"
)

// ImageAliasResourceModel resource data model that matches the schema.
type ImageAliasResourceModel struct {
	Name        types.String `tfsdk:"name"`
	Fingerprint types.String `tfsdk:"fingerprint"`
	Description types.String `tfsdk:"description"`
	Project     types.String `tfsdk:"project"`
	Remote      types.String `tfsdk:"remote"`

	// Computed.
	ResourceID types.String `tfsdk:"resource_id"`
}

// ImageAliasResource represents an Incus image alias resource.
type ImageAliasResource struct {
	provider *provider_config.IncusProviderConfig
}

// NewImageAliasResource returns a new image alias resource.
func NewImageAliasResource() resource.Resource {
	return &ImageAliasResource{}
}

func (r ImageAliasResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = fmt.Sprintf("%s_image_alias", req.ProviderTypeName)
}

func (r ImageAliasResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Incus image alias.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the image alias",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},

			"fingerprint": schema.StringAttribute{
				Required:    true,
				Description: "Fingerprint of the image this alias should point to",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},

			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the image alias",
			},

			"project": schema.StringAttribute{
				Optional:    true,
				Description: "Name of the project where the image alias will be created",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},

			"remote": schema.StringAttribute{
				Optional:    true,
				Description: "The remote in which the resource will be created. If not provided, the default remote is used",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},

			// Computed attributes
			"resource_id": schema.StringAttribute{
				Computed:    true,
				Description: "Unique identifier for this resource",
			},
		},
	}
}

func (r *ImageAliasResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data := req.ProviderData
	if data == nil {
		return
	}

	provider, ok := data.(*provider_config.IncusProviderConfig)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider_config.IncusProviderConfig, got: %T", data),
		)
		return
	}

	r.provider = provider
}

func (r ImageAliasResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ImageAliasResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	remote, project := r.resolveRemoteProject(plan)
	server, err := r.provider.InstanceServer(remote, project, "")
	if err != nil {
		resp.Diagnostics.Append(errors.NewInstanceServerError(err))
		return
	}

	aliasName := plan.Name.ValueString()
	fingerprint := plan.Fingerprint.ValueString()
	description := plan.Description.ValueString()

	// Check if the image with this fingerprint exists
	_, _, err = server.GetImage(fingerprint)
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Image with fingerprint %q not found", fingerprint),
			err.Error(),
		)
		return
	}

	// Check if alias already exists
	existingAlias, _, err := server.GetImageAlias(aliasName)
	if err == nil {
		// Alias exists - always error and require explicit import
		resp.Diagnostics.AddError(
			fmt.Sprintf("Alias %q already exists", aliasName),
			fmt.Sprintf("Alias %q already exists and points to fingerprint %q. "+
				"To manage it with Terraform, import it first:\n\n"+
				"  terraform import incus_image_alias.<resource_name> %s",
				aliasName, existingAlias.Target, aliasName),
		)
		return
	} else if !errors.IsNotFoundError(err) {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Failed to check if alias %q exists", aliasName),
			err.Error(),
		)
		return
	}

	// Alias doesn't exist, create it
	aliasPost := api.ImageAliasesPost{}
	aliasPost.Name = aliasName
	aliasPost.Description = description
	aliasPost.Target = fingerprint

	err = server.CreateImageAlias(aliasPost)
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Failed to create alias %q for image %q", aliasName, fingerprint),
			err.Error(),
		)
		return
	}

	// Set resource ID
	plan.Remote = types.StringValue(remote)
	plan.Project = types.StringValue(project)
	plan.ResourceID = types.StringValue(r.resourceID(remote, project, aliasName))

	// Update Terraform state
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r ImageAliasResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ImageAliasResourceModel

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	remote, project := r.resolveRemoteProject(state)
	server, err := r.provider.InstanceServer(remote, project, "")
	if err != nil {
		resp.Diagnostics.Append(errors.NewInstanceServerError(err))
		return
	}

	aliasName := state.Name.ValueString()

	// Get the alias
	alias, _, err := server.GetImageAlias(aliasName)
	if err != nil {
		if errors.IsNotFoundError(err) {
			// Alias doesn't exist, remove from state
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			fmt.Sprintf("Failed to read alias %q", aliasName),
			err.Error(),
		)
		return
	}

	// Update state with current values
	state.Fingerprint = types.StringValue(alias.Target)
	state.Description = types.StringValue(alias.Description)
	state.Remote = types.StringValue(remote)
	state.Project = types.StringValue(project)
	state.ResourceID = types.StringValue(r.resourceID(remote, project, aliasName))

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r ImageAliasResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ImageAliasResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	remote, project := r.resolveRemoteProject(plan)
	server, err := r.provider.InstanceServer(remote, project, "")
	if err != nil {
		resp.Diagnostics.Append(errors.NewInstanceServerError(err))
		return
	}

	aliasName := plan.Name.ValueString()
	fingerprint := plan.Fingerprint.ValueString()
	description := plan.Description.ValueString()

	// Check if the target image exists
	_, _, err = server.GetImage(fingerprint)
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Image with fingerprint %q not found", fingerprint),
			err.Error(),
		)
		return
	}

	// Get current alias to retrieve ETag for optimistic concurrency control
	_, etag, err := server.GetImageAlias(aliasName)
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Failed to retrieve alias %q for update", aliasName),
			err.Error(),
		)
		return
	}

	// Update the alias
	aliasUpdate := api.ImageAliasesEntryPut{
		Description: description,
		Target:      fingerprint,
	}

	err = server.UpdateImageAlias(aliasName, aliasUpdate, etag)
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Failed to update alias %q", aliasName),
			err.Error(),
		)
		return
	}

	// Set resource ID
	plan.Remote = types.StringValue(remote)
	plan.Project = types.StringValue(project)
	plan.ResourceID = types.StringValue(r.resourceID(remote, project, aliasName))

	// Update Terraform state
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

func (r ImageAliasResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ImageAliasResourceModel

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	remote, project := r.resolveRemoteProject(state)
	server, err := r.provider.InstanceServer(remote, project, "")
	if err != nil {
		resp.Diagnostics.Append(errors.NewInstanceServerError(err))
		return
	}

	aliasName := state.Name.ValueString()

	// Delete the alias
	err = server.DeleteImageAlias(aliasName)
	if err != nil && !errors.IsNotFoundError(err) {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Failed to delete alias %q", aliasName),
			err.Error(),
		)
		return
	}
}

func (r ImageAliasResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	meta := common.ImportMetadata{
		ResourceName:   "image_alias",
		RequiredFields: []string{"name"},
	}

	fields, diag := meta.ParseImportID(req.ID)
	if diag != nil {
		resp.Diagnostics.Append(diag)
		return
	}

	remote := fields["remote"]
	project := fields["project"]
	name := fields["name"]

	if remote == "" {
		remote = r.provider.DefaultRemote()
	}

	if project == "" {
		project = r.provider.DefaultProject()
	}

	resourceID := r.resourceID(remote, project, name)

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("remote"), remote)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("resource_id"), resourceID)...)
}

func (r ImageAliasResource) resolveRemoteProject(model ImageAliasResourceModel) (string, string) {
	remote := model.Remote.ValueString()
	project := model.Project.ValueString()

	if remote == "" && r.provider != nil {
		remote = r.provider.DefaultRemote()
	}

	if project == "" && r.provider != nil {
		project = r.provider.DefaultProject()
	}

	return remote, project
}

func (r ImageAliasResource) resourceID(remote, project, alias string) string {
	resolvedRemote := remote
	resolvedProject := project

	if resolvedRemote == "" && r.provider != nil {
		resolvedRemote = r.provider.DefaultRemote()
	}

	if resolvedProject == "" && r.provider != nil {
		resolvedProject = r.provider.DefaultProject()
	}

	return fmt.Sprintf("%s/%s/%s", resolvedRemote, resolvedProject, alias)
}
