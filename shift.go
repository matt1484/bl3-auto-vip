package bl3_auto_vip

import (
	"errors"
	"strings"
	"time"
)

type ShiftConfig struct {
	CodeListUrl string `json:"codeListUrl"`
	CodeInfoUrl string `json:"codeInfoUrl"`
	UserInfoUrl string `json:"userInfoUrl"`
	GameCodename string `json:"gameCodename"`
	AllowInactive bool
}

type ShiftCodeMap map[string][]string

func (codeMap ShiftCodeMap) Contains(code, platform string) bool {
	platforms, found := codeMap[code]
	if !found {
		return false
	}
	for _, p := range platforms {
		if p == platform {
			return true
		}
	}
	return false
}

type shiftCode struct {
	Game string `json:"offer_title"`
	Platform string `json:"offer_service"`
	Active bool `json:"is_active"`
}

type shiftCodeFromList struct {
	Code string `json:"code"`
	Platform string `json:"platform"`
}

func (client *Bl3Client) GetCodePlatforms(code string) ([]string, bool) {
	platforms := make([]string, 0)

	res, err := client.Get(client.Config.Shift.CodeInfoUrl + code + "/info")
	if err != nil {
		return platforms, false
	}

	json, err := res.BodyAsJson()
	if err != nil {
		return platforms, false
	}

	codes := make([]shiftCode, 0)
	json.From("entitlement_offer_codes").Select("offer_service", "is_active", "offer_title").Out(&codes)
	for _, code := range codes {
		if (code.Active || client.Config.Shift.AllowInactive) && code.Game == client.Config.Shift.GameCodename {
			platforms = append(platforms, code.Platform)
		}
	}

	if len(platforms) == 0 {
		return platforms, false
	}

	return platforms, true
}

func (client *Bl3Client) RedeemShiftCode(code, platform string) error {
	response, err := client.Post(client.Config.Shift.CodeInfoUrl + code + "/redeem/" + platform, "", nil)
	if err != nil {
		return errors.New("failed to initialize code redemption.")
	}

	type redemptionJob struct {
		JobId string `json:"job_id"`
		Wait int `json:"max_wait_milliseconds"`
	}

	resJson, err := response.BodyAsJson()
	if err != nil {
		return errors.New("bad code init response.")
	}

	redemptionInfo := redemptionJob{}
	resJson.Out(&redemptionInfo)

	if redemptionInfo.JobId == "" {
		redemptionError := ""
		resJson.Reset().From("error.code").Out(&redemptionError)
		if redemptionError != "" {
			return errors.New(strings.ToLower(strings.Join(strings.Split(redemptionError, "_"), " ")) + ". Try again later.")
		}
		return errors.New("failed to schedule code redemption.")
	}
	// not sure if this is necessary
	time.Sleep(time.Duration(redemptionInfo.Wait) * time.Millisecond)

	redeemResponse, err := client.Get(client.Config.Shift.CodeInfoUrl + code + "/job/" + redemptionInfo.JobId)
	if err != nil {
		return errors.New("failed to initialize code redemption.")
	}
	
	resJson, err = redeemResponse.BodyAsJson()
	if err != nil {
		return errors.New("bad code redemption response.")
	}

	success := false
	resJson.From("success").Out(&success)
	errs := make([]string, 0)
	resJson.Reset().From("errors").Out(&errs)
	if len(errs) > 0 {
		return errors.New(strings.ToLower(strings.Join(strings.Split(errs[0], "_"), " ")) + ".")
	}
	if !success {
		return errors.New("failed to redeem shift code.")
	}
	
	resJson.Out(&redemptionInfo)

	return nil
}

func (client *Bl3Client) GetShiftPlatforms() (StringSet, error) {
	platforms := StringSet{}

	response, err := client.Post(client.Config.Shift.UserInfoUrl, "", nil)
	if err != nil {
		return platforms, errors.New("Failed to get available platforms list")
	}

	resJson, err := response.BodyAsJson()
	if err != nil {
		return platforms, err
	}

	platformList := make([]string, 0)
	resJson.From("platforms").Out(&platformList)

	for _, platform := range platformList {
		platforms.Add(platform)
	}
	return platforms, nil
}

func (client *Bl3Client) GetFullShiftCodeList() (ShiftCodeMap, error) {
	codeMap := ShiftCodeMap{}
	httpClient, err := NewHttpClient()
	if err != nil {
		return codeMap, err
	}

	res, err := httpClient.Get(client.Config.Shift.CodeListUrl)
	if err != nil {
		return codeMap, errors.New("Failed to get SHIFT code list")
	}

	json, err := res.BodyAsJson()
	if err != nil {
		return codeMap, errors.New("Failed to get SHIFT code list body as JSON")
	}

	codes := make([]shiftCodeFromList, 0)
	json.From("[0].codes").Select("code", "platform").Out(&codes)
	for _, code := range codes {
		platforms, valid := client.GetCodePlatforms(code.Code)
		if valid {
			codeMap[code.Code] = platforms
		}
	}

	return codeMap, nil
}