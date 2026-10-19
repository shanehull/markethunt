// Package country holds the reference table that ties Damodaran's country labels
// to ISO 3166-1 alpha-3 codes and World Bank regions, so equity multiples and
// macro series can be joined on a single key.
package country

import "slices"

// Info is one country in the reference table.
type Info struct {
	ISO3   string
	Name   string
	Region string
}

// All is every market Damodaran values that also has a stable ISO3 code. The
// names are Damodaran's labels, which read more naturally than the World Bank's
// ("South Korea" not "Korea, Rep.").
var All = []Info{
	{ISO3: "ARE", Name: "United Arab Emirates", Region: "Middle East & North Africa"},
	{ISO3: "ARG", Name: "Argentina", Region: "Latin America & Caribbean"},
	{ISO3: "AUS", Name: "Australia", Region: "East Asia & Pacific"},
	{ISO3: "AUT", Name: "Austria", Region: "Europe & Central Asia"},
	{ISO3: "AZE", Name: "Azerbaijan", Region: "Europe & Central Asia"},
	{ISO3: "BEL", Name: "Belgium", Region: "Europe & Central Asia"},
	{ISO3: "BEN", Name: "Benin", Region: "Sub-Saharan Africa"},
	{ISO3: "BFA", Name: "Burkina Faso", Region: "Sub-Saharan Africa"},
	{ISO3: "BGD", Name: "Bangladesh", Region: "South Asia"},
	{ISO3: "BGR", Name: "Bulgaria", Region: "Europe & Central Asia"},
	{ISO3: "BHR", Name: "Bahrain", Region: "Middle East & North Africa"},
	{ISO3: "BHS", Name: "Bahamas", Region: "Latin America & Caribbean"},
	{ISO3: "BMU", Name: "Bermuda", Region: "North America"},
	{ISO3: "BRA", Name: "Brazil", Region: "Latin America & Caribbean"},
	{ISO3: "BRB", Name: "Barbados", Region: "Latin America & Caribbean"},
	{ISO3: "BWA", Name: "Botswana", Region: "Sub-Saharan Africa"},
	{ISO3: "CAN", Name: "Canada", Region: "North America"},
	{ISO3: "CHE", Name: "Switzerland", Region: "Europe & Central Asia"},
	{ISO3: "CHL", Name: "Chile", Region: "Latin America & Caribbean"},
	{ISO3: "CHN", Name: "China", Region: "East Asia & Pacific"},
	{ISO3: "CIV", Name: "Ivory Coast", Region: "Sub-Saharan Africa"},
	{ISO3: "COL", Name: "Colombia", Region: "Latin America & Caribbean"},
	{ISO3: "CRI", Name: "Costa Rica", Region: "Latin America & Caribbean"},
	{ISO3: "CUW", Name: "Curaçao", Region: "Latin America & Caribbean"},
	{ISO3: "CYM", Name: "Cayman Islands", Region: "Latin America & Caribbean"},
	{ISO3: "CYP", Name: "Cyprus", Region: "Europe & Central Asia"},
	{ISO3: "CZE", Name: "Czech Republic", Region: "Europe & Central Asia"},
	{ISO3: "DEU", Name: "Germany", Region: "Europe & Central Asia"},
	{ISO3: "DNK", Name: "Denmark", Region: "Europe & Central Asia"},
	{ISO3: "DOM", Name: "Dominican Republic", Region: "Latin America & Caribbean"},
	{ISO3: "ECU", Name: "Ecuador", Region: "Latin America & Caribbean"},
	{ISO3: "EGY", Name: "Egypt", Region: "Middle East & North Africa"},
	{ISO3: "ESP", Name: "Spain", Region: "Europe & Central Asia"},
	{ISO3: "EST", Name: "Estonia", Region: "Europe & Central Asia"},
	{ISO3: "FIN", Name: "Finland", Region: "Europe & Central Asia"},
	{ISO3: "FRA", Name: "France", Region: "Europe & Central Asia"},
	{ISO3: "GAB", Name: "Gabon", Region: "Sub-Saharan Africa"},
	{ISO3: "GBR", Name: "United Kingdom", Region: "Europe & Central Asia"},
	{ISO3: "GGY", Name: "Guernsey", Region: "Europe & Central Asia"},
	{ISO3: "GHA", Name: "Ghana", Region: "Sub-Saharan Africa"},
	{ISO3: "GIB", Name: "Gibraltar", Region: "Europe & Central Asia"},
	{ISO3: "GRC", Name: "Greece", Region: "Europe & Central Asia"},
	{ISO3: "HKG", Name: "Hong Kong", Region: "East Asia & Pacific"},
	{ISO3: "HRV", Name: "Croatia", Region: "Europe & Central Asia"},
	{ISO3: "HUN", Name: "Hungary", Region: "Europe & Central Asia"},
	{ISO3: "IDN", Name: "Indonesia", Region: "East Asia & Pacific"},
	{ISO3: "IMN", Name: "Isle of Man", Region: "Europe & Central Asia"},
	{ISO3: "IND", Name: "India", Region: "South Asia"},
	{ISO3: "IRL", Name: "Ireland", Region: "Europe & Central Asia"},
	{ISO3: "ISL", Name: "Iceland", Region: "Europe & Central Asia"},
	{ISO3: "ISR", Name: "Israel", Region: "Middle East & North Africa"},
	{ISO3: "ITA", Name: "Italy", Region: "Europe & Central Asia"},
	{ISO3: "JAM", Name: "Jamaica", Region: "Latin America & Caribbean"},
	{ISO3: "JEY", Name: "Jersey", Region: "Europe & Central Asia"},
	{ISO3: "JOR", Name: "Jordan", Region: "Middle East & North Africa"},
	{ISO3: "JPN", Name: "Japan", Region: "East Asia & Pacific"},
	{ISO3: "KAZ", Name: "Kazakhstan", Region: "Europe & Central Asia"},
	{ISO3: "KEN", Name: "Kenya", Region: "Sub-Saharan Africa"},
	{ISO3: "KHM", Name: "Cambodia", Region: "East Asia & Pacific"},
	{ISO3: "KOR", Name: "South Korea", Region: "East Asia & Pacific"},
	{ISO3: "KWT", Name: "Kuwait", Region: "Middle East & North Africa"},
	{ISO3: "LBR", Name: "Liberia", Region: "Sub-Saharan Africa"},
	{ISO3: "LCA", Name: "Saint Lucia", Region: "Latin America & Caribbean"},
	{ISO3: "LIE", Name: "Liechtenstein", Region: "Europe & Central Asia"},
	{ISO3: "LKA", Name: "Sri Lanka", Region: "South Asia"},
	{ISO3: "LTU", Name: "Lithuania", Region: "Europe & Central Asia"},
	{ISO3: "LUX", Name: "Luxembourg", Region: "Europe & Central Asia"},
	{ISO3: "LVA", Name: "Latvia", Region: "Europe & Central Asia"},
	{ISO3: "MAC", Name: "Macau", Region: "East Asia & Pacific"},
	{ISO3: "MAR", Name: "Morocco", Region: "Middle East & North Africa"},
	{ISO3: "MCO", Name: "Monaco", Region: "Europe & Central Asia"},
	{ISO3: "MDA", Name: "Moldova", Region: "Europe & Central Asia"},
	{ISO3: "MEX", Name: "Mexico", Region: "Latin America & Caribbean"},
	{ISO3: "MHL", Name: "Marshall Islands", Region: "East Asia & Pacific"},
	{ISO3: "MKD", Name: "Macedonia", Region: "Europe & Central Asia"},
	{ISO3: "MLT", Name: "Malta", Region: "Europe & Central Asia"},
	{ISO3: "MMR", Name: "Myanmar", Region: "East Asia & Pacific"},
	{ISO3: "MNE", Name: "Montenegro", Region: "Europe & Central Asia"},
	{ISO3: "MNG", Name: "Mongolia", Region: "East Asia & Pacific"},
	{ISO3: "MTQ", Name: "Martinique", Region: "Latin America & Caribbean"},
	{ISO3: "MUS", Name: "Mauritius", Region: "Sub-Saharan Africa"},
	{ISO3: "MWI", Name: "Malawi", Region: "Sub-Saharan Africa"},
	{ISO3: "MYS", Name: "Malaysia", Region: "East Asia & Pacific"},
	{ISO3: "NAM", Name: "Namibia", Region: "Sub-Saharan Africa"},
	{ISO3: "NGA", Name: "Nigeria", Region: "Sub-Saharan Africa"},
	{ISO3: "NLD", Name: "Netherlands", Region: "Europe & Central Asia"},
	{ISO3: "NOR", Name: "Norway", Region: "Europe & Central Asia"},
	{ISO3: "NZL", Name: "New Zealand", Region: "East Asia & Pacific"},
	{ISO3: "OMN", Name: "Oman", Region: "Middle East & North Africa"},
	{ISO3: "PAK", Name: "Pakistan", Region: "Middle East & North Africa"},
	{ISO3: "PAN", Name: "Panama", Region: "Latin America & Caribbean"},
	{ISO3: "PER", Name: "Peru", Region: "Latin America & Caribbean"},
	{ISO3: "PHL", Name: "Philippines", Region: "East Asia & Pacific"},
	{ISO3: "PNG", Name: "Papua New Guinea", Region: "East Asia & Pacific"},
	{ISO3: "POL", Name: "Poland", Region: "Europe & Central Asia"},
	{ISO3: "PRT", Name: "Portugal", Region: "Europe & Central Asia"},
	{ISO3: "PSE", Name: "Palestinian Authority", Region: "Middle East & North Africa"},
	{ISO3: "QAT", Name: "Qatar", Region: "Middle East & North Africa"},
	{ISO3: "ROU", Name: "Romania", Region: "Europe & Central Asia"},
	{ISO3: "RUS", Name: "Russia", Region: "Europe & Central Asia"},
	{ISO3: "SAU", Name: "Saudi Arabia", Region: "Middle East & North Africa"},
	{ISO3: "SDN", Name: "Sudan", Region: "Sub-Saharan Africa"},
	{ISO3: "SEN", Name: "Senegal", Region: "Sub-Saharan Africa"},
	{ISO3: "SGP", Name: "Singapore", Region: "East Asia & Pacific"},
	{ISO3: "SLV", Name: "El Salvador", Region: "Latin America & Caribbean"},
	{ISO3: "SRB", Name: "Serbia", Region: "Europe & Central Asia"},
	{ISO3: "SVK", Name: "Slovakia", Region: "Europe & Central Asia"},
	{ISO3: "SVN", Name: "Slovenia", Region: "Europe & Central Asia"},
	{ISO3: "SWE", Name: "Sweden", Region: "Europe & Central Asia"},
	{ISO3: "TCA", Name: "Turks & Caicos Islands", Region: "Latin America & Caribbean"},
	{ISO3: "THA", Name: "Thailand", Region: "East Asia & Pacific"},
	{ISO3: "TTO", Name: "Trinidad & Tobago", Region: "Latin America & Caribbean"},
	{ISO3: "TUN", Name: "Tunisia", Region: "Middle East & North Africa"},
	{ISO3: "TUR", Name: "Turkey", Region: "Europe & Central Asia"},
	{ISO3: "TWN", Name: "Taiwan", Region: "East Asia & Pacific"},
	{ISO3: "TZA", Name: "Tanzania", Region: "Sub-Saharan Africa"},
	{ISO3: "UGA", Name: "Uganda", Region: "Sub-Saharan Africa"},
	{ISO3: "UKR", Name: "Ukraine", Region: "Europe & Central Asia"},
	{ISO3: "URY", Name: "Uruguay", Region: "Latin America & Caribbean"},
	{ISO3: "USA", Name: "United States", Region: "North America"},
	{ISO3: "VEN", Name: "Venezuela", Region: "Latin America & Caribbean"},
	{ISO3: "VGB", Name: "British Virgin Islands", Region: "Latin America & Caribbean"},
	{ISO3: "VNM", Name: "Vietnam", Region: "East Asia & Pacific"},
	{ISO3: "ZAF", Name: "South Africa", Region: "Sub-Saharan Africa"},
	{ISO3: "ZMB", Name: "Zambia", Region: "Sub-Saharan Africa"},
	{ISO3: "ZWE", Name: "Zimbabwe", Region: "Sub-Saharan Africa"},
}

// ByDamodaranName indexes the table by Damodaran's country label.
var ByDamodaranName = func() map[string]Info {
	m := make(map[string]Info, len(All))
	for _, c := range All {
		m[c.Name] = c
	}
	return m
}()

// ByISO3 indexes the table by ISO3 code.
var ByISO3 = func() map[string]Info {
	m := make(map[string]Info, len(All))
	for _, c := range All {
		m[c.ISO3] = c
	}
	return m
}()

// Regions returns the distinct regions in a stable display order.
func Regions() []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range All {
		if !seen[c.Region] {
			seen[c.Region] = true
			out = append(out, c.Region)
		}
	}
	slices.Sort(out)
	return out
}
