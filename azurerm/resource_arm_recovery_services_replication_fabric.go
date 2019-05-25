package azurerm

import (
	"fmt"
	"log"
	"regexp"

	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/helpers/tf"

	"github.com/Azure/azure-sdk-for-go/services/recoveryservices/mgmt/2018-01-10/siterecovery"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
)

func resourceArmRecoveryServicesReplicationFabric() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmRecoveryServicesReplicationFabricCreate,
		Read:   resourceArmRecoveryServicesReplicationFabricRead,
		Delete: resourceArmRecoveryServicesReplicationFabricDelete,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringMatch(
					regexp.MustCompile("^[a-zA-Z][-a-zA-Z0-9]{1,49}$"), // TODO: Determine actual name restrictions on Azure
					"Replication Fabric name must be 2 - 50 characters long, start with a letter, contain only letters, numbers and hyphens.",
				),
			},

			"location": locationSchema(),

			"resource_group_name": resourceGroupNameSchema(),

			"vault_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringMatch(
					regexp.MustCompile("^[a-zA-Z][-a-zA-Z0-9]{1,49}$"),
					"Recovery Service Vault name must be 2 - 50 characters long, start with a letter, contain only letters, numbers and hyphens.",
				),
			},
		},
	}
}

func resourceArmRecoveryServicesReplicationFabricCreate(d *schema.ResourceData, meta interface{}) error {
	ctx := meta.(*ArmClient).StopContext

	name := d.Get("name").(string)
	vault := d.Get("vault_name").(string)
	resourceGroup := d.Get("resource_group_name").(string)
	location := d.Get("location").(string)

	client := meta.(*ArmClient).getReplicationFabricClientForRecoveryServicesVault(resourceGroup, vault)

	log.Printf("[DEBUG] Creating/updating Recovery Service Replication Fabric %q (resource group %q, vault %q)", name, resourceGroup, vault)

	if requireResourcesToBeImported && d.IsNewResource() {
		existing, err := client.Get(ctx, name)
		if err != nil {
			if !utils.ResponseWasNotFound(existing.Response) {
				return fmt.Errorf("Error checking for presence of existing Recovery Service Replication Fabric %q (Resource Group %q, vault %q): %+v", name, resourceGroup, name, err)
			}
		}

		if existing.ID != nil && *existing.ID != "" {
			return tf.ImportAsExistsError("azurerm_recovery_services_replication_fabric", *existing.ID)
		}
	}

	// Build custom input for Azure fabric
	azureInput := siterecovery.AzureFabricCreationInput{
		Location:     &location,
		InstanceType: siterecovery.InstanceTypeAzure,
	}

	//build fabric struct
	fabric := siterecovery.FabricCreationInput{
		Properties: &siterecovery.FabricCreationInputProperties{
			CustomDetails: azureInput,
		},
	}

	//create recovery services vault
	future, err := client.Create(ctx, name, fabric)
	if err != nil {
		return fmt.Errorf("Error creating/updating Recovery Service Vault %q (Resource Group %q, Vault %q): %+v", name, vault, resourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("Error creating/updating Recovery Service Vault %q (Resource Group %q, Vault %q): %+v", name, vault, resourceGroup, err)
	}

	result, err := future.Result(*client)

	if err != nil {
		return fmt.Errorf("Error creating/updating Recovery Service Vault %q (Resource Group %q, Vault %q): %+v", name, vault, resourceGroup, err)
	}

	d.SetId(*result.ID)

	return resourceArmRecoveryServicesReplicationFabricRead(d, meta)
}

func resourceArmRecoveryServicesReplicationFabricRead(d *schema.ResourceData, meta interface{}) error {
	id, err := parseAzureResourceID(d.Id())
	if err != nil {
		return err
	}

	vault := id.Path["vaults"]
	name := id.Path["replicationFabrics"]
	resourceGroup := id.ResourceGroup

	client := meta.(*ArmClient).getReplicationFabricClientForRecoveryServicesVault(resourceGroup, vault)
	ctx := meta.(*ArmClient).StopContext

	log.Printf("[DEBUG] Reading Recovery Service Vault %q (resource group %q)", name, resourceGroup)

	resp, err := client.Get(ctx, name)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			d.SetId("")
			return nil
		}

		return fmt.Errorf("Error making Read request on Recovery Service Replication Fabric %q (Resource Group %q, Vault %q): %+v", name, resourceGroup, vault, err)
	}

	d.Set("name", resp.Name)
	d.Set("resource_group_name", resourceGroup)
	d.Set("vault_name", vault)
	azureDetails, isAzure := resp.Properties.CustomDetails.AsAzureFabricSpecificDetails()
	if isAzure {
		if location := azureDetails.Location; location != nil {
			d.Set("location", azureRMNormalizeLocation(*location))
		}
	}

	return nil
}

func resourceArmRecoveryServicesReplicationFabricDelete(d *schema.ResourceData, meta interface{}) error {
	id, err := parseAzureResourceID(d.Id())
	if err != nil {
		return err
	}

	vault := id.Path["vaults"]
	name := id.Path["replicationFabrics"]
	resourceGroup := id.ResourceGroup

	client := meta.(*ArmClient).getReplicationFabricClientForRecoveryServicesVault(resourceGroup, vault)
	ctx := meta.(*ArmClient).StopContext

	log.Printf("[DEBUG] Deleting Recovery Service Replication Fabric %q (resource group %q, vault %q)", name, vault, resourceGroup)

	future, err := client.Delete(ctx, name)
	if err != nil {
		return fmt.Errorf("Error issuing delete request for Recovery Service Replication Fabric %q (Resource Group %q, Vault %q): %+v", name, vault, resourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("Error issuing delete request for Recovery Service Replication Fabric %q (Resource Group %q, Vault %q): %+v", name, vault, resourceGroup, err)
	}

	resp, err := future.Result(*client)

	if err != nil {
		if !utils.ResponseWasNotFound(resp) {
			return fmt.Errorf("Deletion request failed for Recovery Service Replication Fabric %q (Resource Group %q, Vault %q): %+v", name, vault, resourceGroup, err)
		}
	}

	return nil
}
