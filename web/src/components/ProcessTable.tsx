import { useState, useMemo } from "react"
import { ArrowUp, ArrowDown } from "lucide-react"
import { formatPercent } from "@/lib/format"
import type { Process } from "@/hooks/useMonitorState"
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from "@/components/ui/table"

type SortColumn = "pid" | "user" | "cpuPct" | "memPct" | "command"
type SortDirection = "asc" | "desc"

const COLUMN_HEADERS: { key: SortColumn; label: string }[] = [
  { key: "pid", label: "PID" },
  { key: "user", label: "User" },
  { key: "cpuPct", label: "CPU%" },
  { key: "memPct", label: "Memory%" },
  { key: "command", label: "Command" },
]

interface ProcessTableProps {
  processes: Process[]
}

export function ProcessTable({ processes }: ProcessTableProps) {
  const [sortColumn, setSortColumn] = useState<SortColumn>("cpuPct")
  const [sortDirection, setSortDirection] = useState<SortDirection>("desc")

  const sortedProcesses = useMemo(() => {
    const top20 = processes.slice(0, 20)
    return [...top20].sort((a, b) => {
      const aVal = a[sortColumn]
      const bVal = b[sortColumn]
      if (typeof aVal === "string" && typeof bVal === "string") {
        return sortDirection === "asc"
          ? aVal.localeCompare(bVal)
          : bVal.localeCompare(aVal)
      }
      const aNum = aVal as number
      const bNum = bVal as number
      return sortDirection === "asc" ? aNum - bNum : bNum - aNum
    })
  }, [processes, sortColumn, sortDirection])

  if (sortedProcesses.length === 0) {
    return <p className="text-sm text-muted-foreground">No process data available.</p>
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          {COLUMN_HEADERS.map(({ key, label }) => (
            <TableHead
              key={key}
              className="cursor-pointer select-none"
              onClick={() => {
                if (sortColumn === key) {
                  setSortDirection((d) => (d === "asc" ? "desc" : "asc"))
                } else {
                  setSortColumn(key)
                  setSortDirection(key === "user" || key === "command" ? "asc" : "desc")
                }
              }}
            >
              <span className="inline-flex items-center gap-1">
                {label}
                {sortColumn === key && (
                  sortDirection === "asc"
                    ? <ArrowUp className="size-3" />
                    : <ArrowDown className="size-3" />
                )}
              </span>
            </TableHead>
          ))}
        </TableRow>
      </TableHeader>
      <TableBody>
        {sortedProcesses.map((proc) => (
          <TableRow key={proc.pid}>
            <TableCell>{proc.pid}</TableCell>
            <TableCell>{proc.user}</TableCell>
            <TableCell>{formatPercent(proc.cpuPct)}</TableCell>
            <TableCell>{formatPercent(proc.memPct)}</TableCell>
            <TableCell>{proc.command}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
