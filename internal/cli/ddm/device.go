package ddm

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"path"
	"strings"

	"net/http"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/macadmins/nanohubctl/internal/utils"
)

// deviceCmd manages set membership for a given device
func deviceCmd() *cobra.Command {
	deviceCmd := &cobra.Command{
		Use:     "device",
		Short:   "Device related operations",
		Long:    "Device related operations",
		PreRunE: utils.ApplyPreExecFn,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.Help(); err != nil {
				return err
			}
			return nil
		},
	}

	deviceCmd.AddCommand(
		addDeviceCmd(),
		getDeviceCmd(),
		removeDeviceCmd(),
		declarationStatusCmd(),
		errorsCmd(),
		valuesCmd(),
	)

	return deviceCmd
}

// getDeviceCmd retreives all sets applied to a given device
func getDeviceCmd() *cobra.Command {
	getCmd := &cobra.Command{
		Use:     "sets",
		Short:   "Get all sets for a given device",
		Long:    "Get all sets for a given device",
		PreRunE: utils.ApplyPreExecFn,
		RunE:    getdeviceFn,
	}

	return getCmd
}

func getdeviceFn(cmd *cobra.Command, args []string) error {
	deviceID := viper.GetString("client_id")
	ddmUrl, err := utils.GetDDMUrl()
	if err != nil {
		return err
	}
	ddmUrl.Path = path.Join(ddmUrl.Path, "enrollment-sets", deviceID)
	var resp *http.Response
	err = getReq(ddmUrl.String(), &resp)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	var jsonResponse interface{}
	if err := json.Unmarshal(body, &jsonResponse); err != nil {
		return err
	}
	fmt.Println(utils.PrettyJsonPrint(jsonResponse))
	return nil
}

// addDeviceCmd applies a given set to the provided device ID
func addDeviceCmd() *cobra.Command {
	addDeviceCmd := &cobra.Command{
		Use:     "add",
		Short:   "Add a device to a declaration set",
		Long:    "Add a device to a declaration set",
		Args:    cobra.ExactArgs(1),
		PreRunE: utils.ApplyPreExecFn,
		RunE:    addDeviceFn,
	}

	return addDeviceCmd
}

func addDeviceFn(cmd *cobra.Command, args []string) error {
	deviceID := viper.GetString("client_id")

	set := args[0]

	resp, err := addOrDeletedeviceItem("add", deviceID, set)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotModified:
		fmt.Printf("%s is already in %s\n", deviceID, set)
	case http.StatusNoContent:
		fmt.Printf("%s has been added to %s\n", deviceID, set)
	default:
		fmt.Println(resp.Status)
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalln(err)
		}
		return fmt.Errorf(string(body))
	}

	return nil
}

// removeDeviceCmd removes a specified device ID from a given set
func removeDeviceCmd() *cobra.Command {
	removeDeviceCmd := &cobra.Command{
		Use:     "remove",
		Short:   "Remove device from an enrollment set",
		Long:    "Remove device from an enrollment set",
		Args:    cobra.ExactArgs(1),
		PreRunE: utils.ApplyPreExecFn,
		RunE:    removeDeviceFn,
	}

	return removeDeviceCmd
}

func removeDeviceFn(cmd *cobra.Command, args []string) error {
	deviceID := viper.GetString("client_id")

	set := args[0]

	fmt.Printf("Removing device %s from set %s...\n", deviceID, set)

	resp, err := addOrDeletedeviceItem("remove", deviceID, set)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotModified:
		fmt.Printf("%s is not in set: %s\n", deviceID, set)
	case http.StatusNoContent:
		fmt.Printf("%s has been removed from %s\n", deviceID, set)
	default:
		if resp.StatusCode == http.StatusInternalServerError {
			return fmt.Errorf("Set does not exist\n")
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalln(err)
		}
		return fmt.Errorf(string(body))
	}

	return nil
}

// addOrDeletedeviceItem handles http for add and remove, probably better to just duplicate the code. Oh well.
func addOrDeletedeviceItem(action, deviceID, set string) (*http.Response, error) {
	ddmUrl, err := utils.GetDDMUrl()
	if err != nil {
		return nil, err
	}
	// Device path for the enrollment set
	ddmUrl.Path = path.Join(ddmUrl.Path, "enrollment-sets", deviceID)
	// Add the query arguments
	q := ddmUrl.Query()
	q.Set("set", set)
	ddmUrl.RawQuery = q.Encode()
	var resp *http.Response
	var respErr error
	if action == "add" {
		respErr = putReq(ddmUrl.String(), &resp)
	} else if action == "remove" {
		respErr = deleteReq(ddmUrl.String(), &resp)
	}
	if respErr != nil {
		return nil, respErr
	}
	return resp, nil
}

func declarationStatusCmd() *cobra.Command {
	declarationStatusCmd := &cobra.Command{
		Use:     "declarations [--client-id $ID]",
		Short:   "List declarations for a specified device ID",
		Long:    "List declarations for a specified device ID",
		PreRunE: utils.ApplyPreExecFn,
		RunE:    StatusFn,
	}

	return declarationStatusCmd
}

// errorsCmd Lists errors for a specified device ID
func errorsCmd() *cobra.Command {
	errorsCmd := &cobra.Command{
		Use:     "errors [--client-id $ID]",
		Short:   "List errors for a specified device ID",
		Long:    "List errors for a specified device ID",
		PreRunE: utils.ApplyPreExecFn,
		RunE:    StatusFn,
	}

	return errorsCmd
}

// valuesCmd lists all values for a specified device ID
func valuesCmd() *cobra.Command {
	valuesCmd := &cobra.Command{
		Use:     "values [--client-id $ID]",
		Short:   "List values for a specified device ID",
		Long:    "List values for a specified device ID",
		PreRunE: utils.ApplyPreExecFn,
		RunE:    StatusFn,
	}

	return valuesCmd
}

// StatusFn handles all logic for the various status commands
func StatusFn(cmd *cobra.Command, statuss []string) error {
	clientID := viper.GetString("client_id")
	ddmUrl, err := utils.GetDDMUrl()
	if err != nil {
		return err
	}
	cmdVerb := strings.Split(cmd.Use, " ")[0]
	switch cmdVerb {
	case "declarations":
		ddmUrl.Path = path.Join(ddmUrl.Path, "declaration-status", clientID)
	case "values":
		ddmUrl.Path = path.Join(ddmUrl.Path, "status-values", clientID)
	case "errors":
		ddmUrl.Path = path.Join(ddmUrl.Path, "status-errors", clientID)
	default:
		return fmt.Errorf("%s is not a valid status type", cmdVerb)
	}
	var resp *http.Response
	err = getReq(ddmUrl.String(), &resp)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	var jsonResponse interface{}
	if err := json.Unmarshal(body, &jsonResponse); err != nil {
		return err
	}
	fmt.Println(utils.PrettyJsonPrint(jsonResponse))
	return nil
}
