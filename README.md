# Archived

Please note:

This repository has been migrated to [Codeberg](https://codeberg.org/dazfuller/azcosts) and so has been archived at this location

# Azure Costs CLI

This is a small CLI app I wrote as I wanted to get back into writing code with Go. This tool allows users to collect costs for a monthly billing period broken down by resource group and persisted locally. The app can then be used to generate a report showing the month-by-month spend per resource group in either text, csv, json, or Excel formats.

The application uses the same APIs as the billing blade in the Azure Portal.

## Subscriptions

The application is capable of list the Azure Subscriptions the account has access to, if a name is provided then the list of subscriptions is filtered by fuzzy matching the input name against the subscriptions display name.

## Collecting

When collecting billing data you can specify the following arguments.

| Argument     | Required | Description                                                      |
|--------------|----------|------------------------------------------------------------------|
| subscription | No       | The GUID value of the subscription to collect for                |
| name         | No       | The full or partial name of the subscription to collect for      |
| year         | No       | The billing year to collect for (default is the current year)    |
| month        | No       | The billing month to collect for (default is the current month)  |
| overwrite    | No       | When used will re-collect the billing data for the current month |
| truncate     | No       | When used will truncate all data collected so far                |

Either the subscription id or name _must_ be specified. Where a name is specified then if a single subscription is found it will be collected immediately. If more than 1 subscription is found the user is prompted to confirm which subscription they wish to collect for.

If data has already been collected for the subscription and the provided billing period it will not be re-collected until the `-overwrite` flag is provided.

Example usage

```bash
> azcosts collect -subscription <subscription id> -year 2024 -month 2
```

The APIs have a low usage policy and so rapid requests to collect data may result in throttling issues, the application will attempt 3 times to collect the data and obeys the retry wait period specified by the API.

## Generating reports

The application can generate pivoted reports showing resource group billing information with billing periods shown in their own columns. The available export formats are text, csv, json, and Excel.

When generating the following arguments are available.


| Argument | Required | Description                                                                   |
|----------|----------|-------------------------------------------------------------------------------|
| format   | No       | The type of format to use for the generated output                            |
| stdout   | No       | If specified then the report is written to stdout (not available for Excel)   |
| path     | No       | When not writing to stdout a path must be specified to generate the report at |
| months   | No       | The number of months to export in the generated report                        |

Example usage

```bash
> azcosts generate -format text -stdout

Resource Group                      Subscription                        2023-10      2023-11      2023-12      2024-01      2024-02 Total Costs
=================================== ============================== ============ ============ ============ ============ ============ ============
ResourceGroup1                      My Subscription                      239.76       264.56       124.58         5.32         3.29       637.51
ResourceGroup2                      My Subscription                        8.24         7.97         6.21         7.82         5.44        35.68
ResourceGroup3                      My Subscription                        0.71         0.00         0.00         0.00         0.00         0.71
ResourceGroup4                      My Subscription                        0.00         0.00         0.00         0.00         0.00         0.01
ResourceGroup5                      My Subscription                        0.00         4.74        20.86        28.25        18.51        72.36
```
