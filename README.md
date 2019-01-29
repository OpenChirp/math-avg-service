[![Build Status](https://travis-ci.org/OpenChirp/math-avg-service.svg?branch=master)](https://travis-ci.org/OpenChirp/math-avg-service)

# Math Average OpenChirp Service

## Overview
This service applies a simple sliding window average.

# Service Config
| Key Name | Key Description | Key Example | Is Required? |
| - | - | - | - |
| `InputTopics` | Comma separated list of input topics | temp,frequency | Required |
| `OutputTopics` | Comma separated list of corresponding output topics | temp_avg, frequency_avg | Optional |
| `WindowSizes` | Comma separated list of corresponding window sizes | 2, 4 | Optional |
  (in number of samples) | Optional |

# Design Decision
The decision has been made to allow producing startup averages with less than
the specified window size in order to always give the user output.
The alternative approach would have been to wait for the window to become full
before we could generate our first averaged output.
