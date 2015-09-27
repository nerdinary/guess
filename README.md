Guess stuff!
============

You have a UNIX timestamp and don’t know which date it equals? You have a date
without timezone and want to see where it probably originates? You have a
number of bytes that looks suspiciously like 23 Terabytes, but you want to be
sure? Good thing there’s `guess`!

See it in action:

    $ ./guess 1443346122085
    Timestamp 1443346122085 is 2015-09-27 19:28:42.085 +1000 AEST (within the day, 4 hours 22 minutes ago)
        In other time zones:
        2015-09-27 02:28:42.085 -0700 PDT (America/Los_Angeles)
        2015-09-27 05:28:42.085 -0400 EDT (America/New_York)
        2015-09-27 09:28:42.085 +0000 UTC (UTC)
        2015-09-27 11:28:42.085 +0200 CEST (Europe/Berlin)
        2015-09-27 13:28:42.085 +0400 GST (Asia/Dubai)
        2015-09-27 17:28:42.085 +0800 SGT (Asia/Singapore)
        2015-09-27 19:28:42.085 +1000 AEST (Australia/Sydney)
        UNIX timestamp: 1443346122


    $ ./guess 8TiB
    8796093022208 bytes
        8589934592.0 KiB (8796093022.2 KB)
        8388608.0 MiB (8796093.0 MB)
        8192.0 GiB (8796.1 GB)
        8.0 TiB (8.8 TB)



    $ ./guess 2001:4860:4860::8888
    IP address 2001:4860:4860::8888
        reverse lookup: google-public-dns-a.google.com.
        which resolves to: 8.8.8.8, 2001:4860:4860::8888


    $ ./guess "2015-09-25 15:00:00"
    In local time: 2015-09-25 15:00:00 +1000 AEST (within the week, 2 days 8 hours 51 minutes 44 seconds ago)
        From PDT (America/Los_Angeles): 2015-09-26 08:00:00 +1000 AEST       September 2015
        From EDT (America/New_York): 2015-09-26 05:00:00 +1000 AEST       Mo Tu We Th Fr Sa Su
        From UTC (UTC): 2015-09-26 01:00:00 +1000 AEST                        1  2  3  4  5  6
        From CEST (Europe/Berlin): 2015-09-25 23:00:00 +1000 AEST          7  8  9 10 11 12 13
        From GST (Asia/Dubai): 2015-09-25 21:00:00 +1000 AEST             14 15 16 17 18 19 20
        From SGT (Asia/Singapore): 2015-09-25 17:00:00 +1000 AEST         21 22 23 24 25 26 27
        From AEST (Australia/Sydney): 2015-09-25 15:00:00 +1000 AEST      28 29 30
