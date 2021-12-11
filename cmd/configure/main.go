package configure

import (
	"bufio"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/cloudfoundry/jibber_jabber"
	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/server"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/templates"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed localization/de.toml
var lang_de string

//go:embed localization/en.toml
var lang_en string

type CmdConfigure struct {
	configuration Configure
	localizer     *i18n.Localizer
	log           *util.Logger

	lang                                 string
	advancedMode, expandedMode           bool
	addedDeviceIndex                     int
	errItemNotPresent, errDeviceNotValid error
}

// Run starts the interactive configuration
func (c *CmdConfigure) Run(log *util.Logger, flagLang string, advancedMode, expandedMode bool) {
	c.log = log
	c.advancedMode = advancedMode
	c.expandedMode = expandedMode

	c.log.INFO.Printf("evcc %s (%s)", server.Version, server.Commit)

	bundle := i18n.NewBundle(language.German)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	if _, err := bundle.ParseMessageFileBytes([]byte(lang_de), "localization/de.toml"); err != nil {
		panic(err)
	}
	if _, err := bundle.ParseMessageFileBytes([]byte(lang_en), "localization/en.toml"); err != nil {
		panic(err)
	}

	c.lang = "de"
	systemLanguage, err := jibber_jabber.DetectLanguage()
	if err == nil {
		c.lang = systemLanguage
	}
	if flagLang != "" {
		c.lang = flagLang
	}

	c.localizer = i18n.NewLocalizer(bundle, c.lang)

	c.setDefaultTexts()

	fmt.Println()
	fmt.Println(c.localizedString("Intro", nil))
	flowChoices := []string{
		c.localizedString("Flow_Type_NewConfiguration", nil),
		c.localizedString("Flow_Type_SingleDevice", nil),
	}
	fmt.Println()
	flowIndex, _ := c.askChoice(c.localizedString("Flow_Type", nil), flowChoices)
	switch flowIndex {
	case 0:
		c.flowNewConfigFile()
	case 1:
		c.flowSingleDevice()
	}
}

// configureSingleDevice implements the flow for getting a single device configuration
func (c *CmdConfigure) flowSingleDevice() {
	fmt.Println()
	fmt.Println(c.localizedString("Flow_SingleDevice_Setup", nil))
	fmt.Println()
	fmt.Println(c.localizedString("Flow_SingleDevice_Select", nil))

	// only consider the device categories that are marked for this flow
	categoryChoices := []string{
		DeviceCategories[DeviceCategoryGridMeter].title,
		DeviceCategories[DeviceCategoryPVMeter].title,
		DeviceCategories[DeviceCategoryBatteryMeter].title,
		DeviceCategories[DeviceCategoryChargeMeter].title,
		DeviceCategories[DeviceCategoryCharger].title,
		DeviceCategories[DeviceCategoryVehicle].title,
	}

	fmt.Println()
	_, cagetoryTitle := c.askChoice(c.localizedString("Flow_SingleDevice_Select", nil), categoryChoices)

	var selectedCategory DeviceCategory
	for item, data := range DeviceCategories {
		if data.title == cagetoryTitle {
			selectedCategory = item
			break
		}
	}

	devices := c.configureDevices(selectedCategory, false, false)
	for _, item := range devices {
		fmt.Println()
		fmt.Println(c.localizedString("Flow_SingleDevice_Config", localizeMap{}))
		fmt.Println()

		scanner := bufio.NewScanner(strings.NewReader(item.Yaml))
		for scanner.Scan() {
			fmt.Println("  " + scanner.Text())
		}
	}
	fmt.Println()
}

// configureNewConfigFile implements the flow for creating a new configuration file
func (c *CmdConfigure) flowNewConfigFile() {
	fmt.Println()
	fmt.Println(c.localizedString("Flow_NewConfiguration_Setup", nil))
	fmt.Println()
	fmt.Println(c.localizedString("Flow_NewConfiguration_Select", localizeMap{"ItemNotPresent": c.localizedString("ItemNotPresent", nil)}))
	c.configureDeviceGuidedSetup()

	_ = c.configureDevices(DeviceCategoryGridMeter, true, false)
	_ = c.configureDevices(DeviceCategoryPVMeter, true, true)
	_ = c.configureDevices(DeviceCategoryBatteryMeter, true, true)
	_ = c.configureDevices(DeviceCategoryVehicle, true, true)
	c.configureLoadpoints()
	c.configureSite()

	yaml, err := c.configuration.RenderConfiguration()
	if err != nil {
		c.log.FATAL.Fatal(err)
	}

	fmt.Println()

	filename := DefaultConfigFilename

	for ok := true; ok; {
		_, err := os.Open(filename)
		if errors.Is(err, os.ErrNotExist) {
			break
		}

		if c.askYesNo(c.localizedString("File_Exists", localizeMap{"FileName": filename})) {
			break
		}

		filename = c.askValue(question{
			label:        c.localizedString("File_NewFilename", nil),
			exampleValue: "evcc_neu.yaml",
			required:     true})
	}

	err = os.WriteFile(filename, yaml, 0755)
	if err != nil {
		fmt.Printf("%s: ", c.localizedString("File_Error_SaveFailed", localizeMap{"FileName": filename}))
		c.log.FATAL.Fatal(err)
	}
	fmt.Println(c.localizedString("File_SaveSuccess", localizeMap{"FileName": filename}))
}

// configureDevices asks device specfic questions
func (c *CmdConfigure) configureDevices(deviceCategory DeviceCategory, askAdding, askMultiple bool) []device {
	var devices []device

	if deviceCategory == DeviceCategoryGridMeter && c.configuration.MetersOfCategory(deviceCategory) > 0 {
		return nil
	}

	localizeMap := localizeMap{
		"Article":    DeviceCategories[deviceCategory].article,
		"Additional": DeviceCategories[deviceCategory].additional,
		"Category":   DeviceCategories[deviceCategory].title,
	}
	if askAdding {
		addDeviceText := c.localizedString("AddDeviceInCategory", localizeMap)
		if c.configuration.MetersOfCategory(deviceCategory) > 0 {
			addDeviceText = c.localizedString("AddAnotherDeviceInCategory", localizeMap)
		}

		fmt.Println()
		if !c.askYesNo(addDeviceText) {
			return nil
		}
	}

	for ok := true; ok; {
		device, err := c.configureDeviceCategory(deviceCategory)
		if err != nil {
			break
		}
		devices = append(devices, device)

		if !askMultiple {
			break
		}

		fmt.Println()
		if !c.askYesNo(c.localizedString("AddAnotherDeviceInCategory", localizeMap)) {
			break
		}
	}

	return devices
}

// configureLoadpoints asks loadpoint specific questions
func (c *CmdConfigure) configureLoadpoints() {
	fmt.Println()
	fmt.Println(c.localizedString("Loadpoint_Setup", nil))

	for ok := true; ok; {

		loadpointTitle := c.askValue(question{
			label:        c.localizedString("Loadpoint_Title", nil),
			defaultValue: c.localizedString("Loadpoint_DefaultTitle", nil),
			required:     true})
		loadpoint := loadpoint{
			Title:      loadpointTitle,
			Phases:     3,
			MinCurrent: 6,
		}

		charger, err := c.configureDeviceCategory(DeviceCategoryCharger)
		if err != nil {
			break
		}
		chargerHasMeter := charger.ChargerHasMeter

		loadpoint.Charger = charger.Name

		if !chargerHasMeter {
			if c.askYesNo(c.localizedString("Loadpoint_WallboxWOMeter", nil)) {
				chargeMeter, err := c.configureDeviceCategory(DeviceCategoryChargeMeter)
				if err == nil {
					loadpoint.ChargeMeter = chargeMeter.Name
					chargerHasMeter = true
				}
			}
		}

		vehicles := c.configuration.DevicesOfClass(DeviceClassVehicle)
		if len(vehicles) == 1 {
			loadpoint.Vehicles = append(loadpoint.Vehicles, vehicles[0].Name)
		} else if len(vehicles) > 1 {
			for _, vehicle := range vehicles {
				if c.askYesNo(c.localizedString("Loadpoint_VehicleChargeHere", localizeMap{"Vehicle": vehicle.Title})) {
					loadpoint.Vehicles = append(loadpoint.Vehicles, vehicle.Name)
				}
			}
		}

		powerChoices := []string{
			c.localizedString("Loadpoint_WallboxPower36kW", nil),
			c.localizedString("Loadpoint_WallboxPower11kW", nil),
			c.localizedString("Loadpoint_WallboxPower22kW", nil),
			c.localizedString("Loadpoint_WallboxPowerOther", nil),
		}
		fmt.Println()
		powerIndex, _ := c.askChoice(c.localizedString("Loadpoint_WallboxMaxPower", nil), powerChoices)
		switch powerIndex {
		case 0:
			loadpoint.MaxCurrent = 16
			if !chargerHasMeter {
				loadpoint.Phases = 1
			}
		case 1:
			loadpoint.MaxCurrent = 16
			if !chargerHasMeter {
				loadpoint.Phases = 3
			}
		case 2:
			loadpoint.MaxCurrent = 32
			if !chargerHasMeter {
				loadpoint.Phases = 3
			}
		case 3:
			amperage := c.askValue(question{
				label:     c.localizedString("Loadpoint_WallboxMaxAmperage", nil),
				valueType: templates.ParamValueTypeNumber,
				required:  true})
			loadpoint.MaxCurrent, _ = strconv.Atoi(amperage)

			if !chargerHasMeter {
				phaseChoices := []string{"1", "2", "3"}
				fmt.Println()
				phaseIndex, _ := c.askChoice(c.localizedString("Loadpoint_WallboxPhases", nil), phaseChoices)
				loadpoint.Phases = phaseIndex + 1
			}
		}

		chargingModes := []string{string(api.ModeOff), string(api.ModeNow), string(api.ModeMinPV), string(api.ModePV)}
		chargeModes := []string{
			c.localizedString("Loadpoint_ChargeModeOff", nil),
			c.localizedString("Loadpoint_ChargeModeNow", nil),
			c.localizedString("Loadpoint_ChargeModeMinPV", nil),
			c.localizedString("Loadpoint_ChargeModePV", nil),
		}
		fmt.Println()
		modeChoice, _ := c.askChoice(c.localizedString("Loadpoint_DefaultChargeMode", nil), chargeModes)
		loadpoint.Mode = chargingModes[modeChoice]

		c.configuration.AddLoadpoint(loadpoint)

		fmt.Println()
		if !c.askYesNo(c.localizedString("Loadpoint_AddAnother", nil)) {
			break
		}
	}
}

// configureSite asks site specific questions
func (c *CmdConfigure) configureSite() {
	fmt.Println()
	fmt.Println(c.localizedString("Site_Setup", nil))

	siteTitle := c.askValue(question{
		label:        c.localizedString("Site_Title", nil),
		defaultValue: c.localizedString("Site_DefaultTitle", nil),
		required:     true})
	c.configuration.config.Site.Title = siteTitle
}