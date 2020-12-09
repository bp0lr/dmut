## dmut

### what?

A tool written in golang to perform permutations, mutations and alteration of subdomains and brute force the result.


### why?

I'm doing some work on automatization for bug bounty, and I found myself needing something to brute force for new subdomains using these techniques.

Doing some research I found altdns, a tool that does what I need but written in python.

Speed is everything in bug bounty, usually, you have many subdomains to scan so I put myself in the task of writing a new tool that did the same as altdns but focused on speed and adding some improvements to the complete process.


### type of permutations, mutations, alterations.

The main subdomain is **a.b.com**
from a word list, where you have for example the word **stage**, dmut will generate and try for a positive response:

- stagea.b.com
- astage.b.com
- stage.a.b.com
- a.stage.b.com
- stage-a.b.com
- a-stage.b.com


### dns servers

To get the best from **dmut**, you need a DNS server list.


Using [dnsFaster](https://github.com/bp0lr/dnsfaster), I have created a github action to run this tool again a public list generated from (https://public-dns.info/nameserver/us.txt).


this action runs one time a day and update the repo automatically.

You can download this list from the repo [dmut-resolvers](https://github.com/bp0lr/dmut-resolvers) or running **dmut** with the flag --update-dnslist to update your local copy.

```
dmut --update-dnslist
```
and the new list would be saved to /~/.dmut/resolvers.txt


it's really important to have your list in the best shape possible. The resolution times varied from one DNS server to another, you have some server doing DNS hijacking for some domains or responding with errors after several connections.
Be careful and take your time to test your list.


### Speed

**dmut** is significantly much faster than his python brother.

I did some tests to compare his speed using the same options and an accurate DNS server list.

```
root@dnsMaster# time python3 altdns.py -i list.txt -o data_output -r -w words.txt -t 100 -f /root/.dmut/resolvers.txt -s results.txt
...
real    9m44.712s
user    7m7.741s
sys     1m6.288s

root@dnsMaster# wc -l results.txt
55
```

```
root@dnsMaster# time cat list.txt | dmut -w 100 -d words.txt --dns-retries 3 -o results.txt -s /root/.dmut/resolvers.txt --dns-errorLimit 50 --dns-timeout 350 --show-stats
...
real    5m31.318s
user    1m4.024s
sys     0m41.876s

root@dnsMaster# wc -l results.txt
55
```

If you run the same test but using a default DNS server list downloaded from public-dns.info, the difference is just too much.
Here is where the anti-hijacking, found confirmations, DNS timeout and extra checks come to play in favor of dmut.

```
root@dnsMaster# time python3 altdns.py -i list.txt -o data_output -r -w words.txt -t 100 -f dnsinfo-list.txt -s results.txt
...
real    112m6.295s
user    8m17.104s
sys     1m14.583s
```

```
cat list.txt | ./dmut-binary -w 100 -d words.txt --dns-retries 3 -o results.txt -s dnsinfo-list.txt --dns-errorLimit 10 --dns-timeout 300 --show-stats
real    8m21.627s
user    1m14.191s
sys     0m48.982s
```

just wow!



### Install

Install is quick and clean
```
go get -u github.com/bp0lr/dmut
```

You need a mutations list to make dmut works.

You can use my list downloading the file from [here](https://raw.githubusercontent.com/bp0lr/dmut/main/words.txt)


### examples
```
dmut -u "test.example.com" -d mutations.txt -w 100 --dns-timeout 300 --dns-retries 5 --dns-errorLimit 25 --show-stats -o results.txt
```
this will run **dmut** again test.example.com, using the word list mutations.txt, using 100 workers, having a DNS timeout of 300ms and 5 retries for each error. 
If a DNS server reaches 25 errors, this server is blacklisted and not used again.

Show stats add some verbose to the process.

If we found something would be saved to results.txt

```
cat subdomainList.txt | dmut -d mutations.txt -w 100 --dns-timeout 300 --dns-retries 5 --dns-errorLimit 25 --show-stats -o results.txt
```
the same but using a subdomain list.


### options

```
Usage of dmut:
  -d, --dictionary string    Dictionary file containing mutation list
      --dns-errorLimit int   How many errors until we the DNS is disabled (default 25)
      --dns-retries int      Max amount of retries for failed dns queries (default 3)
      --dns-timeout int      Dns Server timeOut in millisecond (default 500)
  -s, --dnsFile string       Use DNS servers from this file
  -l, --dnsServers string    Use DNS servers from a list separated by ,
  -o, --output string        Output file to save the results to
      --show-ip              Display extra info for valid results
      --show-stats           Display stats about the current job
      --update-dnslist       Download a list of periodically validated public DNS resolvers
  -u, --url string           Target URL
  -v, --verbose              Add verboicity to the process
  -w, --workers int          How many Workers amount (default 25)
```

### Wildcard filtering
**dmut** will test each subdomain for wildcards, requesting a not supposed to exist subdomain.
If we get a positive response the job will be ignored.


### Contributing
Everyone is encouraged to contribute to **dmut** by forking the Github repository and making a pull request or opening an issue.


### AltDNS

altdns was originaly created by **infosec-au** and can be found here (https://github.com/infosec-au/altdns)

Looks like the project was abandoned at some point, so I had forked and did my own version with some improvements. (https://github.com/bp0lr/altdns)

I want to thank **infosec-au** because his work was my inspiration for dmut.
