# Math Average OpenChirp Service

## Overview
This service applies a simple sliding window average.

# Service Config
* `InputTopics` - Required - Comma separated list of input topics
* `OutputTopics` - Optional - Comma separated list of corresponding output topics
* `WindowSizes` - Optional - Comma separated list of corresponding window sizes
  (in number of samples)

# Design Decision
The decision has been made to allow producing startup averages with less than
the specified window size, in order to always give the user output.
The alternative approach would be to wait to the window to become full
before we could generate our first average.