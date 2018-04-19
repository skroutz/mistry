const JobsRoot = document.getElementById('js-jobs')

class Jobs extends React.Component {
  constructor(props) {
      super(props)
      this.every = props.every
      this.state = {}
    };

  fetchJobs() {
    fetch("/index").
      then(response => response.json()).
      then(data => this.setState({ jobs: data }));
  };

  componentDidMount() {
    this.fetchJobs();
    this.interval = setInterval(() => this.fetchJobs(), this.every);
  };

  componentWillUnmount() {
    clearInterval(this.interval);
  };

  render() {
    if (this.state.jobs == undefined) {
      return (
          <div class="jumbotron">
            <h3>No jobs...</h3>
          </div>
      );
    }

    let jobs = this.state.jobs;
    return (
      <table class="hover unstriped">
        <thead>
          <tr>
          <th>ID</th>
          <th>Project</th>
          <th>Started At</th>
          <th>State</th>
          </tr>
        </thead>
        <tbody>
          {jobs.map(function(j, idx){
            return (
              <tr key={idx}>
                <td><a href={`/job/${j.project}/${j.id}`} > {j.id} </a></td>
                <td>{j.project}</td>
                <td>{j.startedAt}</td>
                <td>{j.state}</td>
              </tr>
            )
           })}
        </tbody>
      </table>
    );
  }
}

ReactDOM.render(<Jobs every={3000} />, JobsRoot)
