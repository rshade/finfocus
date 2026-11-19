# Data model

- A chart point contains a checkpoint version, timestamp, total, and provider
  values from the history response.
- The viewport is a start index and point count. Its bounds stay within the
  applied date and currency range.
- One deployment selection identifies a checkpoint by version. Its preceding
  checkpoint supplies the starting version for the existing diff request.
- Provider and threshold visibility are scoped to the chart instance.
