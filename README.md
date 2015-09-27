Guess stuff!
============

You have a UNIX timestamp and don’t know which date it equals? You have a date
without timezone and want to see where it probably originates? You have a
number of bytes that looks suspiciously like 23 Terabytes, but you want to be
sure? Good thing there’s `guess`!

See it in action:

    $ ./guess 1443346122085
    2015-09-27 19:28:42.085 +1000 AEST (within the minute, 53 seconds ago)
        In other time zones:
        2015-09-27 02:28:42.085 -0700 PDT
        2015-09-27 05:28:42.085 -0400 EDT
        2015-09-27 09:28:42.085 +0000 UTC
        2015-09-27 18:28:42.085 +0900 JST
        UNIX timestamp: 1443346122

    1.3 TiB (1.4 TB)
    1409517697.3 KiB (1443346122.1 KB)
    1344.2 GiB (1443.3 GB)
    1376482.1 MiB (1443346.1 MB)


    $ ./guess 8TiB
    8796093022208 bytes


    $ ./guess 2001:4860:4860::8888
    IP address 2001:4860:4860::8888
        reverse lookup: google-public-dns-a.google.com.
        which resolves to: 8.8.8.8, 2001:4860:4860::8888


    $ ./guess "2015-09-25 15:00:00"
    In local time: 2015-09-25 15:00:00 +1000 AEST (within the week, 2 days 4 hours 33 minutes 24 seconds ago)
        From PDT (America/Los_Angeles): 2015-09-26 08:00:00 +1000 AEST       September 2015
        From EDT (America/New_York): 2015-09-26 05:00:00 +1000 AEST       Mo Tu We Th Fr Sa Su
        From UTC (UTC): 2015-09-26 01:00:00 +1000 AEST                        1  2  3  4  5  6
        From JST (Asia/Tokyo): 2015-09-25 16:00:00 +1000 AEST              7  8  9 10 11 12 13
                                                                          14 15 16 17 18 19 20
                                                                          21 22 23 24 25 26 27
                                                                          28 29 30
