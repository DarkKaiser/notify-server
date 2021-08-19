package task

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/darkkaiser/notify-server/g"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
)

const (
	// TaskID
	TidCovid19 TaskID = "COVID19"

	// TaskCommandID
	TcidCovid19WatchResidualVaccine TaskCommandID = "WatchResidualVaccine" // 코로나19 잔여백신 확인
)

const prefixSelectedMedicalInstitutionOpenURL = "https://m.place.naver.com/rest/vaccine?vaccineFilter=used&selected_place_id="

type covid19WatchResidualVaccineSearchResultData []struct {
	Data struct {
		Rests struct {
			Businesses struct {
				Total           int   `json:"total"`
				VaccineLastSave int64 `json:"vaccineLastSave"`
				IsUpdateDelayed bool  `json:"isUpdateDelayed"`
				Items           []struct {
					ID                   string      `json:"id"`
					Name                 string      `json:"name"`
					DbType               string      `json:"dbType"`
					Phone                string      `json:"phone"`
					VirtualPhone         interface{} `json:"virtualPhone"`
					HasBooking           bool        `json:"hasBooking"`
					HasNPay              bool        `json:"hasNPay"`
					BookingReviewCount   string      `json:"bookingReviewCount"`
					Description          interface{} `json:"description"`
					Distance             string      `json:"distance"`
					CommonAddress        string      `json:"commonAddress"`
					RoadAddress          string      `json:"roadAddress"`
					Address              string      `json:"address"`
					ImageURL             interface{} `json:"imageUrl"`
					ImageCount           int         `json:"imageCount"`
					Tags                 interface{} `json:"tags"`
					PromotionTitle       interface{} `json:"promotionTitle"`
					Category             string      `json:"category"`
					RouteURL             string      `json:"routeUrl"`
					BusinessHours        string      `json:"businessHours"`
					X                    string      `json:"x"`
					Y                    string      `json:"y"`
					IsDelivery           interface{} `json:"isDelivery"`
					IsTakeOut            interface{} `json:"isTakeOut"`
					IsPreOrder           interface{} `json:"isPreOrder"`
					IsTableOrder         interface{} `json:"isTableOrder"`
					NaverBookingCategory interface{} `json:"naverBookingCategory"`
					BookingDisplayName   interface{} `json:"bookingDisplayName"`
					BookingBusinessID    interface{} `json:"bookingBusinessId"`
					BookingVisitID       interface{} `json:"bookingVisitId"`
					BookingPickupID      interface{} `json:"bookingPickupId"`
					VaccineOpeningHour   struct {
						StartTime    string `json:"startTime"`
						EndTime      string `json:"endTime"`
						IsDayOff     bool   `json:"isDayOff"`
						StandardTime string `json:"standardTime"`
						Typename     string `json:"__typename"`
					} `json:"vaccineOpeningHour"`
					VaccineQuantity struct {
						TotalQuantity           int    `json:"totalQuantity"`
						TotalQuantityStatus     string `json:"totalQuantityStatus"`
						VaccineOrganizationCode string `json:"vaccineOrganizationCode"`
						List                    []struct {
							Quantity       int    `json:"quantity"`
							QuantityStatus string `json:"quantityStatus"`
							VaccineType    string `json:"vaccineType"`
							Typename       string `json:"__typename"`
						} `json:"list"`
						Typename string `json:"__typename"`
					} `json:"vaccineQuantity"`
					Typename string `json:"__typename"`
				} `json:"items"`
				Typename string `json:"__typename"`
			} `json:"businesses"`
			QueryResult struct {
				Keyword       string      `json:"keyword"`
				VaccineFilter interface{} `json:"vaccineFilter"`
				Categories    []string    `json:"categories"`
				Region        interface{} `json:"region"`
				IsBrandList   interface{} `json:"isBrandList"`
				FilterBooking interface{} `json:"filterBooking"`
				HasNearQuery  interface{} `json:"hasNearQuery"`
				IsPublicMask  interface{} `json:"isPublicMask"`
				Typename      string      `json:"__typename"`
			} `json:"queryResult"`
			Typename string `json:"__typename"`
		} `json:"rests"`
	} `json:"data"`
}

type covid19MedicalInstitution struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	VaccineQuantity string `json:"vaccine_quantity"`
}

func (mi *covid19MedicalInstitution) String(messageTypeHTML bool, mark string) string {
	if messageTypeHTML == true {
		return fmt.Sprintf("☞ <a href=\"%s%s\"><b>%s</b></a> 잔여백신 %s개%s", prefixSelectedMedicalInstitutionOpenURL, mi.ID, mi.Name, mi.VaccineQuantity, mark)
	}
	return strings.TrimSpace(fmt.Sprintf("☞ %s 잔여백신 %s개%s", mi.Name, mi.VaccineQuantity, mark))
}

type covid19WatchResidualVaccineResultData struct {
	MedicalInstitutions []*covid19MedicalInstitution `json:"medical_institutions"`
}

func init() {
	supportedTasks[TidCovid19] = &supportedTaskConfig{
		commandConfigs: []*supportedTaskCommandConfig{{
			taskCommandID: TcidCovid19WatchResidualVaccine,

			allowMultipleInstances: true,

			newTaskResultDataFn: func() interface{} { return &covid19WatchResidualVaccineResultData{} },
		}},

		newTaskFn: func(instanceID TaskInstanceID, taskRunData *taskRunData, config *g.AppConfig) (taskHandler, error) {
			if taskRunData.taskID != TidCovid19 {
				return nil, errors.New("등록되지 않은 작업입니다.😱")
			}

			task := &covid19Task{
				task: task{
					id:         taskRunData.taskID,
					commandID:  taskRunData.taskCommandID,
					instanceID: instanceID,

					notifierID: taskRunData.notifierID,

					canceled: false,

					runBy: taskRunData.taskRunBy,
				},

				config: config,
			}

			task.runFn = func(taskResultData interface{}, messageTypeHTML bool) (string, interface{}, error) {
				switch task.CommandID() {
				case TcidCovid19WatchResidualVaccine:
					return task.runWatchResidualVaccine(taskResultData, messageTypeHTML)
				}

				return "", nil, ErrNoImplementationForTaskCommand
			}

			return task, nil
		},
	}
}

type covid19Task struct {
	task

	config *g.AppConfig
}

//noinspection GoUnhandledErrorResult
func (t *covid19Task) runWatchResidualVaccine(taskResultData interface{}, messageTypeHTML bool) (message string, changedTaskResultData interface{}, err error) {
	originTaskResultData, ok := taskResultData.(*covid19WatchResidualVaccineResultData)
	if ok == false {
		log.Panic("TaskResultData의 타입 변환이 실패하였습니다.")
	}

	//
	// 잔여백신이 남아있는 의료기관을 검색한다.
	//
	var header = map[string]string{"content-type": "application/json"}
	var searchResultData = covid19WatchResidualVaccineSearchResultData{}
	err = unmarshalFromResponseJSONData("POST", "https://api.place.naver.com/graphql", header, bytes.NewBufferString("[{\"operationName\":\"vaccineList\",\"variables\":{\"input\":{\"keyword\":\"코로나백신위탁의료기관\",\"x\":\"127.672066\",\"y\":\"34.7635133\"},\"businessesInput\":{\"start\":0,\"display\":100,\"deviceType\":\"mobile\",\"x\":\"127.672066\",\"y\":\"34.7635133\",\"bounds\":\"127.6034014;34.7392187;127.7407305;34.7878008\",\"sortingOrder\":\"distance\"},\"isNmap\":false,\"isBounds\":false},\"query\":\"query vaccineList($input: RestsInput, $businessesInput: RestsBusinessesInput, $isNmap: Boolean!, $isBounds: Boolean!) {\\n  rests(input: $input) {\\n    businesses(input: $businessesInput) {\\n      total\\n      vaccineLastSave\\n      isUpdateDelayed\\n      items {\\n        id\\n        name\\n        dbType\\n        phone\\n        virtualPhone\\n        hasBooking\\n        hasNPay\\n        bookingReviewCount\\n        description\\n        distance\\n        commonAddress\\n        roadAddress\\n        address\\n        imageUrl\\n        imageCount\\n        tags\\n        distance\\n        promotionTitle\\n        category\\n        routeUrl\\n        businessHours\\n        x\\n        y\\n        imageMarker @include(if: $isNmap) {\\n          marker\\n          markerSelected\\n          __typename\\n        }\\n        markerLabel @include(if: $isNmap) {\\n          text\\n          style\\n          __typename\\n        }\\n        isDelivery\\n        isTakeOut\\n        isPreOrder\\n        isTableOrder\\n        naverBookingCategory\\n        bookingDisplayName\\n        bookingBusinessId\\n        bookingVisitId\\n        bookingPickupId\\n        vaccineOpeningHour {\\n          isDayOff\\n          standardTime\\n          __typename\\n        }\\n        vaccineQuantity {\\n          totalQuantity\\n          totalQuantityStatus\\n          startTime\\n          endTime\\n          vaccineOrganizationCode\\n          list {\\n            quantity\\n            quantityStatus\\n            vaccineType\\n            __typename\\n          }\\n          __typename\\n        }\\n        __typename\\n      }\\n      optionsForMap @include(if: $isBounds) {\\n        maxZoom\\n        minZoom\\n        includeMyLocation\\n        maxIncludePoiCount\\n        center\\n        __typename\\n      }\\n      __typename\\n    }\\n    queryResult {\\n      keyword\\n      vaccineFilter\\n      categories\\n      region\\n      isBrandList\\n      filterBooking\\n      hasNearQuery\\n      isPublicMask\\n      __typename\\n    }\\n    __typename\\n  }\\n}\\n\"}]"), &searchResultData)
	if err != nil {
		return "", nil, err
	}

	//
	// 잔여백신이 남아있는 의료기관을 추출한다.
	//
	actualityTaskResultData := &covid19WatchResidualVaccineResultData{}

	// 백신 접종을 하러 갈 수 있는 의료기관 ID 목록
	var yeocheonMedicalInstitutionIDs = []string{"13263626", "11482871", "12080253", "19526949", "13589797", "13263571", "1359325699", "10998196", "13263625", "13263595", "168000943", "13263623", "13263618", "19792738", "13263622", "12794279", "19522666", "13178488", "19530337", "13389513", "13263643", "13263639", "1864819000"}

	for _, item := range searchResultData[0].Data.Rests.Businesses.Items {
		if item.VaccineQuantity.TotalQuantity <= 0 {
			continue
		}

		for _, id := range yeocheonMedicalInstitutionIDs {
			if item.ID == id {
				actualityTaskResultData.MedicalInstitutions = append(actualityTaskResultData.MedicalInstitutions, &covid19MedicalInstitution{
					ID:              item.ID,
					Name:            item.Name,
					VaccineQuantity: strconv.Itoa(item.VaccineQuantity.TotalQuantity),
				})
				break
			}
		}
	}

	//
	// 검색된 잔여백신 정보를 확인한다.
	//
	m := ""
	lineSpacing := "\n\n"
	if messageTypeHTML == true {
		lineSpacing = "\n"
	}
	err = eachSourceElementIsInTargetElementOrNot(actualityTaskResultData.MedicalInstitutions, originTaskResultData.MedicalInstitutions, func(selem, telem interface{}) (bool, error) {
		actualityMedicalInstitution, ok1 := selem.(*covid19MedicalInstitution)
		originMedicalInstitution, ok2 := telem.(*covid19MedicalInstitution)
		if ok1 == false || ok2 == false {
			return false, errors.New("selem/telem의 타입 변환이 실패하였습니다.")
		} else {
			if actualityMedicalInstitution.ID == originMedicalInstitution.ID {
				return true, nil
			}
		}
		return false, nil
	}, func(selem, telem interface{}) {
		actualityMedicalInstitution := selem.(*covid19MedicalInstitution)
		originMedicalInstitution := telem.(*covid19MedicalInstitution)

		if m != "" {
			m += lineSpacing
		}
		if actualityMedicalInstitution.VaccineQuantity != originMedicalInstitution.VaccineQuantity {
			m += actualityMedicalInstitution.String(messageTypeHTML, " 🔁")
		} else {
			m += actualityMedicalInstitution.String(messageTypeHTML, "")
		}
	}, func(selem interface{}) {
		actualityMedicalInstitution := selem.(*covid19MedicalInstitution)

		if m != "" {
			m += lineSpacing
		}
		m += actualityMedicalInstitution.String(messageTypeHTML, " 🆕")
	})
	if err != nil {
		return "", nil, err
	}

	// 백신 갯수가 n개에서 0개로 변경된 의료기관 정보도 출력한다.
	err = eachSourceElementIsInTargetElementOrNot(originTaskResultData.MedicalInstitutions, actualityTaskResultData.MedicalInstitutions, func(selem, telem interface{}) (bool, error) {
		originMedicalInstitution, ok1 := selem.(*covid19MedicalInstitution)
		actualityMedicalInstitution, ok2 := telem.(*covid19MedicalInstitution)
		if ok1 == false || ok2 == false {
			return false, errors.New("selem/telem의 타입 변환이 실패하였습니다.")
		} else {
			if originMedicalInstitution.ID == actualityMedicalInstitution.ID {
				return true, nil
			}
		}
		return false, nil
	}, nil, func(selem interface{}) {
		originMedicalInstitution := selem.(*covid19MedicalInstitution)

		if m != "" {
			m += lineSpacing
		}
		originMedicalInstitution.VaccineQuantity = "0"
		m += originMedicalInstitution.String(messageTypeHTML, " 🔁")
	})
	if err != nil {
		return "", nil, err
	}

	if m != "" {
		message = "코로나19 잔여백신에 대한 정보는 아래와 같습니다:\n\n" + m
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.runBy == TaskRunByUser {
			if len(actualityTaskResultData.MedicalInstitutions) == 0 {
				message = fmt.Sprintf("코로나19 잔여백신이 없습니다.")
			} else {
				for _, actualityMedicalInstitution := range actualityTaskResultData.MedicalInstitutions {
					if m != "" {
						m += lineSpacing
					}
					m += actualityMedicalInstitution.String(messageTypeHTML, "")
				}

				message = "코로나19 잔여백신에 대한 정보는 아래와 같습니다:\n\n" + m
			}
		}
	}

	return message, changedTaskResultData, nil
}
