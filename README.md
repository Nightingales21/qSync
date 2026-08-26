# qSync
qSync fixes rsync's core flaw — one lost packet stalling an entire file batch — by giving each file its own independent QUIC stream over a single connection, so only the affected file slows down while everything else keeps flowing.
